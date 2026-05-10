package attempts

import (
	"context"
	"math"

	"quizsystem/internal/apierr"
	"quizsystem/internal/current"
	"quizsystem/internal/store"

	ent "quizsystem/internal/store/ent"
	entanswers "quizsystem/internal/store/ent/answers"
	entattempt "quizsystem/internal/store/ent/attempt"
	entattemptanswers "quizsystem/internal/store/ent/attempt_answers"
	entquestions "quizsystem/internal/store/ent/questions"
	entquiz "quizsystem/internal/store/ent/quiz"
	entuser "quizsystem/internal/store/ent/user"
)

//encore:api auth method=POST path=/quizzes/:id/submit
func SubmitQuiz(ctx context.Context, id int64, req *SubmitQuizRequest) (*QuizResultResponse, error) {
	user, err := current.RequireRole("user")
	if err != nil {
		return nil, err
	}
	if len(req.Answers) == 0 {
		return nil, apierr.Invalid("answers are required")
	}

	tx, err := store.EntClient.Tx(ctx)
	if err != nil {
		return nil, apierr.Internal("could not start transaction", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	q, err := tx.Quiz.
		Query().
		Where(
			entquiz.IDEQ(int(id)),
			entquiz.IsPublishedEQ(true),
		).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, apierr.NotFound("quiz not found")
		}
		return nil, apierr.Internal("could not load quiz", err)
	}

	if q.OneAttempt {
		existing, err := tx.Attempt.
			Query().
			Where(
				entattempt.HasQuizWith(entquiz.IDEQ(int(id))),
				entattempt.HasUserWith(entuser.IDEQ(int(user.ID))),
			).
			Count(ctx)
		if err != nil {
			return nil, apierr.Internal("could not check previous attempts", err)
		}
		if existing > 0 {
			return nil, apierr.FailedPrecondition("this quiz only allows one attempt")
		}
	}

	questionAnswers, err := loadScoringKey(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	total := len(questionAnswers)
	if total == 0 {
		return nil, apierr.FailedPrecondition("quiz has no questions")
	}

	score, err := scoreSubmission(req.Answers, questionAnswers, total)
	if err != nil {
		return nil, err
	}

	attempt, err := tx.Attempt.
		Create().
		SetScore(score).
		SetTotal(total).
		SetQuizID(int(id)).
		SetUserID(int(user.ID)).
		Save(ctx)
	if err != nil {
		return nil, apierr.Internal("could not create attempt", err)
	}

	for _, answer := range req.Answers {
		_, err := tx.Attempt_answers.
			Create().
			SetAttemptID(attempt.ID).
			SetQuestionID(int(answer.QuestionID)).
			SetAnswerID(int(answer.AnswerID)).
			Save(ctx)
		if err != nil {
			return nil, apierr.Internal("could not save submitted answer", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, apierr.Internal("could not commit attempt", err)
	}
	committed = true

	return loadAttemptResult(ctx, int64(attempt.ID), user.ID)
}

//encore:api auth method=GET path=/quizzes/:id/result
func GetLatestResult(ctx context.Context, id int64) (*QuizResultResponse, error) {
	user, err := current.RequireRole("user")
	if err != nil {
		return nil, err
	}

	attempt, err := store.EntClient.Attempt.
		Query().
		Where(
			entattempt.HasQuizWith(
				entquiz.IDEQ(int(id)),
				entquiz.IsPublishedEQ(true),
			),
			entattempt.HasUserWith(entuser.IDEQ(int(user.ID))),
		).
		Order(entattempt.ByCreatedAt()).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, apierr.NotFound("result not found")
		}
		return nil, apierr.Internal("could not load result", err)
	}

	return loadAttemptResult(ctx, int64(attempt.ID), user.ID)
}

func loadScoringKey(ctx context.Context, tx *ent.Tx, quizID int64) (map[int64]map[int64]bool, error) {
	questions, err := tx.Questions.
		Query().
		Where(entquestions.HasQuizWith(entquiz.IDEQ(int(quizID)))).
		WithAnswers().
		All(ctx)
	if err != nil {
		return nil, apierr.Internal("could not load quiz answers", err)
	}

	key := map[int64]map[int64]bool{}
	for _, q := range questions {
		key[int64(q.ID)] = map[int64]bool{}
		for _, a := range q.Edges.Answers {
			key[int64(q.ID)][int64(a.ID)] = a.IsCorrect
		}
	}

	return key, nil
}

func scoreSubmission(submitted []SubmittedAnswer, key map[int64]map[int64]bool, total int) (int, error) {
	if len(submitted) != total {
		return 0, apierr.Invalid("answer every question before submitting")
	}

	score := 0
	seen := map[int64]bool{}
	for _, item := range submitted {
		answers, ok := key[item.QuestionID]
		if !ok {
			return 0, apierr.Invalid("submitted question does not belong to this quiz")
		}
		if seen[item.QuestionID] {
			return 0, apierr.Invalid("duplicate answer submitted for a question")
		}
		seen[item.QuestionID] = true

		isCorrect, ok := answers[item.AnswerID]
		if !ok {
			return 0, apierr.Invalid("submitted answer does not belong to its question")
		}
		if isCorrect {
			score++
		}
	}

	if len(seen) != total {
		return 0, apierr.Invalid("answer every question before submitting")
	}

	return score, nil
}

func loadAttemptResult(ctx context.Context, attemptID int64, userID int64) (*QuizResultResponse, error) {
	attempt, err := store.EntClient.Attempt.
		Query().
		Where(
			entattempt.IDEQ(int(attemptID)),
			entattempt.HasUserWith(entuser.IDEQ(int(userID))),
		).
		WithQuiz().
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, apierr.NotFound("result not found")
		}
		return nil, apierr.Internal("could not load result", err)
	}

	q := attempt.Edges.Quiz
	result := QuizResultResponse{
		AttemptID:   int64(attempt.ID),
		QuizID:      int64(q.ID),
		QuizTitle:   q.Title,
		Score:       attempt.Score,
		Total:       attempt.Total,
		CreatedAt:   attempt.CreatedAt,
		ShowAnswers: q.ShowAnswers,
	}

	result.Percentage = math.Round((float64(result.Score)/float64(result.Total))*10000) / 100
	if q.PassThreshold != nil {
		passed := result.Percentage >= float64(*q.PassThreshold)
		result.Passed = &passed
	}

	if result.ShowAnswers {
		questions, err := loadAnswerReview(ctx, result.QuizID, result.AttemptID)
		if err != nil {
			return nil, err
		}
		result.Questions = questions
	}

	return &result, nil
}

func loadAnswerReview(ctx context.Context, quizID int64, attemptID int64) ([]QuestionResult, error) {
	questions, err := store.EntClient.Questions.
		Query().
		Where(entquestions.HasQuizWith(entquiz.IDEQ(int(quizID)))).
		Order(entquestions.ByOrderIndex()).
		WithAnswers(func(aq *ent.AnswersQuery) {
			aq.Order(entanswers.ByOrderIndex())
		}).
		All(ctx)
	if err != nil {
		return nil, apierr.Internal("could not load answer review", err)
	}

	// load what the user actually submitted for this attempt
	submitted, err := store.EntClient.Attempt_answers.
		Query().
		Where(entattemptanswers.HasAttemptWith(entattempt.IDEQ(int(attemptID)))).
		WithAnswer().
		WithQuestion().
		All(ctx)
	if err != nil {
		return nil, apierr.Internal("could not load submitted answers", err)
	}

	// index submitted answers by question ID for fast lookup
	submittedByQuestion := map[int64]*ent.Attempt_answers{}
	for _, sa := range submitted {
		submittedByQuestion[int64(sa.Edges.Question.ID)] = sa
	}

	results := make([]QuestionResult, 0)
	for _, question := range questions {
		// find the correct answer
		var correctAnswer *AnswerResult
		for _, a := range question.Edges.Answers {
			if a.IsCorrect {
				correctAnswer = &AnswerResult{ID: int64(a.ID), Text: a.Text}
				break
			}
		}

		qr := QuestionResult{
			QuestionID:    int64(question.ID),
			Text:          question.Text,
			CorrectAnswer: correctAnswer,
		}

		// check if user answered this question
		if sa, ok := submittedByQuestion[int64(question.ID)]; ok {
			userAnswer := sa.Edges.Answer
			qr.UserAnswer = &AnswerResult{ID: int64(userAnswer.ID), Text: userAnswer.Text}
			qr.IsCorrect = correctAnswer != nil && int64(userAnswer.ID) == correctAnswer.ID
		}

		results = append(results, qr)
	}

	return results, nil
}
