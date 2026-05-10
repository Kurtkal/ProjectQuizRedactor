package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/index"
)

type Attempt_answers struct {
	ent.Schema
}

func (Attempt_answers) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("attempt", Attempt.Type).Ref("attempt_answers").Unique().Required(),
		edge.To("question", Questions.Type).Unique().Required(),
		edge.To("answer", Answers.Type).Unique().Required(),
	}
}

func (Attempt_answers) Indexes() []ent.Index {
	return []ent.Index{
		index.Edges("attempt", "question").Unique(),
	}
}
