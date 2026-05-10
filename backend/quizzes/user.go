package quizzes

import (
	"context"

	"quizsystem/internal/apierr"
	"quizsystem/internal/current"
	"quizsystem/internal/store"

	ent "quizsystem/internal/store/ent"
	entanswers "quizsystem/internal/store/ent/answers"
	entattempt "quizsystem/internal/store/ent/attempt"
	entquestions "quizsystem/internal/store/ent/questions"
	entquiz "quizsystem/internal/store/ent/quiz"
	entuser "quizsystem/internal/store/ent/user"
)

//encore:api auth method=GET path=/quizzes
func ListPublishedQuizzes(ctx context.Context) (*QuizListResponse, error) {
	user, err := current.RequireRole("user")
	if err != nil {
		return nil, err
	}

	quizList, err := store.EntClient.Quiz.
		Query().
		Where(entquiz.IsPublishedEQ(true)).
		WithQuestions().
		WithAttempts(func(aq *ent.AttemptQuery) {
			aq.Where(entattempt.HasUserWith(entuser.IDEQ(int(user.ID))))
		}).
		Order(entquiz.ByCreatedAt()).
		All(ctx)
	if err != nil {
		return nil, apierr.Internal("could not list quizzes", err)
	}

	quizzes := make([]QuizSummary, 0)
	for _, q := range quizList {
		summary := QuizSummary{
			ID:            int64(q.ID),
			Title:         q.Title,
			IsPublished:   q.IsPublished,
			QuestionCount: len(q.Edges.Questions),
			OneAttempt:    q.OneAttempt,
			ShowAnswers:   q.ShowAnswers,
			CreatedAt:     q.CreatedAt,
			Completed:     len(q.Edges.Attempts) > 0,
		}
		if q.PassThreshold != nil {
			v := *q.PassThreshold
			summary.PassThreshold = &v
		}
		quizzes = append(quizzes, summary)
	}

	return &QuizListResponse{Quizzes: quizzes}, nil
}

//encore:api auth method=GET path=/quizzes/:id
func GetQuiz(ctx context.Context, id int64) (*PublicQuizDetail, error) {
	user, err := current.RequireRole("user")
	if err != nil {
		return nil, err
	}
	return loadPublicQuiz(ctx, id, user.ID)
}

func loadPublicQuiz(ctx context.Context, id int64, userID int64) (*PublicQuizDetail, error) {
	q, err := store.EntClient.Quiz.
		Query().
		Where(
			entquiz.IDEQ(int(id)),
			entquiz.IsPublishedEQ(true),
		).
		WithQuestions(func(qq *ent.QuestionsQuery) {
			qq.Order(entquestions.ByOrderIndex()).
				WithAnswers(func(aq *ent.AnswersQuery) {
					aq.Order(entanswers.ByOrderIndex())
				})
		}).
		WithAttempts(func(aq *ent.AttemptQuery) {
			aq.Where(entattempt.HasUserWith(entuser.IDEQ(int(userID))))
		}).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, apierr.NotFound("quiz not found")
		}
		return nil, apierr.Internal("could not load quiz", err)
	}

	quiz := PublicQuizDetail{
		ID:         int64(q.ID),
		Title:      q.Title,
		OneAttempt: q.OneAttempt,
		Completed:  len(q.Edges.Attempts) > 0,
	}
	if q.PassThreshold != nil {
		v := *q.PassThreshold
		quiz.PassThreshold = &v
	}

	quiz.Questions = make([]PublicQuestion, 0)
	for _, question := range q.Edges.Questions {
		pq := PublicQuestion{
			ID:      int64(question.ID),
			Text:    question.Text,
			Order:   question.OrderIndex,
			Answers: make([]PublicAnswer, 0),
		}
		for _, answer := range question.Edges.Answers {
			pq.Answers = append(pq.Answers, PublicAnswer{
				ID:    int64(answer.ID),
				Text:  answer.Text,
				Order: answer.OrderIndex,
			})
		}
		quiz.Questions = append(quiz.Questions, pq)
	}

	quiz.QuestionCount = len(quiz.Questions)
	return &quiz, nil
}
