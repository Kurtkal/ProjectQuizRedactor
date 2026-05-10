package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type Attempt struct {
	ent.Schema
}

func (Attempt) Fields() []ent.Field {
	return []ent.Field{
		field.Int("score").Min(0),
		field.Int("total").Positive(),
		field.Time("created_at").Default(time.Now),
	}
}

func (Attempt) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("attempts").Unique().Required(),
		edge.From("quiz", Quiz.Type).Ref("attempts").Unique().Required(),
		edge.To("attempt_answers", Attempt_answers.Type),
	}
}

func (Attempt) Indexes() []ent.Index {
	return []ent.Index{
		index.Edges("user", "quiz").Fields("created_at"),
	}
}
