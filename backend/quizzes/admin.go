package quizzes

import (
	"context"
	"fmt"
	"strings"

	"quizsystem/internal/apierr"
	"quizsystem/internal/current"
	"quizsystem/internal/store"

	ent "quizsystem/internal/store/ent"
	entanswers "quizsystem/internal/store/ent/answers"
	entquestions "quizsystem/internal/store/ent/questions"
	entquiz "quizsystem/internal/store/ent/quiz"
)

type DeleteResponse struct {
	Deleted bool `json:"deleted"`
}

//encore:api auth method=GET path=/admin/quizzes
func ListAdminQuizzes(ctx context.Context) (*QuizListResponse, error) {
	if _, err := current.RequireRole("admin"); err != nil {
		return nil, err
	}

	quizList, err := store.EntClient.Quiz.
		Query().
		WithCreator().
		WithQuestions().
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
		}
		if q.PassThreshold != nil {
			v := *q.PassThreshold
			summary.PassThreshold = &v
		}
		if q.Edges.Creator != nil {
			summary.CreatedBy = &AdminUser{
				ID:    int64(q.Edges.Creator.ID),
				Email: q.Edges.Creator.Email,
			}
		}
		quizzes = append(quizzes, summary)
	}

	return &QuizListResponse{Quizzes: quizzes}, nil
}

//encore:api auth method=POST path=/admin/quizzes
func CreateQuiz(ctx context.Context, req *UpsertQuizRequest) (*AdminQuizDetail, error) {
	fmt.Println("CREATE QUIZ HIT")
	admin, err := current.RequireRole("admin")
	if err != nil {
		return nil, err
	}
	if err := validateQuizInput(req); err != nil {
		return nil, err
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

	qCreate := tx.Quiz.Create().
		SetTitle(strings.TrimSpace(req.Title)).
		SetIsPublished(req.IsPublished).
		SetOneAttempt(req.OneAttempt).
		SetShowAnswers(req.ShowAnswers).
		SetCreatorID(int(admin.ID))
	if req.PassThreshold != nil {
		qCreate = qCreate.SetPassThreshold(*req.PassThreshold)
	}
	q, err := qCreate.Save(ctx)
	if err != nil {
		return nil, apierr.Internal("could not create quiz", err)
	}

	if err := insertQuestionsEnt(ctx, tx, q.ID, req.Questions); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, apierr.Internal("could not commit quiz", err)
	}
	committed = true

	return loadAdminQuiz(ctx, int64(q.ID))
}

//encore:api auth method=GET path=/admin/quizzes/:id
func GetAdminQuiz(ctx context.Context, id int64) (*AdminQuizDetail, error) {
	if _, err := current.RequireRole("admin"); err != nil {
		return nil, err
	}
	return loadAdminQuiz(ctx, id)
}

//encore:api auth method=PUT path=/admin/quizzes/:id
func UpdateQuiz(ctx context.Context, id int64, req *UpsertQuizRequest) (*AdminQuizDetail, error) {
	if _, err := current.RequireRole("admin"); err != nil {
		return nil, err
	}
	if err := validateQuizInput(req); err != nil {
		return nil, err
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

	uUpdate := tx.Quiz.UpdateOneID(int(id)).
		SetTitle(strings.TrimSpace(req.Title)).
		SetIsPublished(req.IsPublished).
		SetOneAttempt(req.OneAttempt).
		SetShowAnswers(req.ShowAnswers)
	if req.PassThreshold != nil {
		uUpdate = uUpdate.SetPassThreshold(*req.PassThreshold)
	} else {
		uUpdate = uUpdate.ClearPassThreshold()
	}
	_, err = uUpdate.Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, apierr.NotFound("quiz not found")
		}
		return nil, apierr.Internal("could not update quiz", err)
	}

	// Delete old questions then reinsert
	_, err = tx.Questions.Delete().
		Where(entquestions.HasQuizWith(entquiz.IDEQ(int(id)))).
		Exec(ctx)
	if err != nil {
		return nil, apierr.Internal("could not replace questions", err)
	}

	if err := insertQuestionsEnt(ctx, tx, int(id), req.Questions); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, apierr.Internal("could not commit quiz", err)
	}
	committed = true

	return loadAdminQuiz(ctx, id)
}

//encore:api auth method=DELETE path=/admin/quizzes/:id
func DeleteQuiz(ctx context.Context, id int64) (*DeleteResponse, error) {
	if _, err := current.RequireRole("admin"); err != nil {
		return nil, err
	}

	err := store.EntClient.Quiz.DeleteOneID(int(id)).Exec(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, apierr.NotFound("quiz not found")
		}
		return nil, apierr.Internal("could not delete quiz", err)
	}
	return &DeleteResponse{Deleted: true}, nil
}

//encore:api auth method=PATCH path=/admin/quizzes/:id/publish
func PublishQuiz(ctx context.Context, id int64, req *PublishRequest) (*AdminQuizDetail, error) {
	if _, err := current.RequireRole("admin"); err != nil {
		return nil, err
	}

	_, err := store.EntClient.Quiz.UpdateOneID(int(id)).
		SetIsPublished(req.IsPublished).
		Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, apierr.NotFound("quiz not found")
		}
		return nil, apierr.Internal("could not change publish status", err)
	}

	return loadAdminQuiz(ctx, id)
}

func insertQuestionsEnt(ctx context.Context, tx *ent.Tx, quizID int, questions []QuestionInput) error {
	for i, question := range questions {
		q, err := tx.Questions.Create().
			SetText(strings.TrimSpace(question.Text)).
			SetOrderIndex(i).
			SetQuizID(quizID).
			Save(ctx)
		if err != nil {
			return apierr.Internal("could not create question", err)
		}

		for j, answer := range question.Answers {
			_, err := tx.Answers.Create().
				SetText(strings.TrimSpace(answer.Text)).
				SetIsCorrect(answer.IsCorrect).
				SetOrderIndex(j).
				SetQuestionID(q.ID).
				Save(ctx)
			if err != nil {
				return apierr.Internal("could not create answer", err)
			}
		}
	}
	return nil
}

func loadAdminQuiz(ctx context.Context, id int64) (*AdminQuizDetail, error) {
	q, err := store.EntClient.Quiz.
		Query().
		Where(entquiz.IDEQ(int(id))).
		WithCreator().
		WithQuestions(func(qq *ent.QuestionsQuery) {
			qq.Order(entquestions.ByOrderIndex()).
				WithAnswers(func(aq *ent.AnswersQuery) {
					aq.Order(entanswers.ByOrderIndex())
				})
		}).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, apierr.NotFound("quiz not found")
		}
		return nil, apierr.Internal("could not load quiz", err)
	}

	quiz := AdminQuizDetail{
		ID:          int64(q.ID),
		Title:       q.Title,
		IsPublished: q.IsPublished,
		OneAttempt:  q.OneAttempt,
		ShowAnswers: q.ShowAnswers,
		CreatedAt:   q.CreatedAt,
		CreatedBy: AdminUser{
			ID:    int64(q.Edges.Creator.ID),
			Email: q.Edges.Creator.Email,
		},
	}
	if q.PassThreshold != nil {
		v := *q.PassThreshold
		quiz.PassThreshold = &v
	}

	quiz.Questions = make([]AdminQuestion, 0)
	for _, question := range q.Edges.Questions {
		aq := AdminQuestion{
			ID:      int64(question.ID),
			Text:    question.Text,
			Order:   question.OrderIndex,
			Answers: make([]AdminAnswer, 0),
		}
		for _, answer := range question.Edges.Answers {
			aq.Answers = append(aq.Answers, AdminAnswer{
				ID:        int64(answer.ID),
				Text:      answer.Text,
				IsCorrect: answer.IsCorrect,
				Order:     answer.OrderIndex,
			})
		}
		quiz.Questions = append(quiz.Questions, aq)
	}

	quiz.QuestionCount = len(quiz.Questions)
	return &quiz, nil
}

func thresholdValue(value *int) interface{} {
	if value == nil {
		return nil
	}
	return *value
}
