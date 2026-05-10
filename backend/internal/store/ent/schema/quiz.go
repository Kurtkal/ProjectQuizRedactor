package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type Quiz struct {
	ent.Schema
}

func (Quiz) Fields() []ent.Field {
	return []ent.Field{
		field.Text("title"),
		field.Bool("is_published").Default(false),
		field.Int("pass_threshold").Optional().Nillable().Range(0, 100),
		field.Bool("one_attempt").Default(false),
		field.Bool("show_answers").Default(false),
		field.Time("created_at").Default(time.Now),
	}
}

func (Quiz) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("creator", User.Type).Ref("quizzes").Unique().Required(),
		edge.To("questions", Questions.Type),
		edge.To("attempts", Attempt.Type),
	}
}

func (Quiz) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("is_published"),
	}
}
