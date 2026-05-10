package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type Questions struct {
	ent.Schema
}

func (Questions) Fields() []ent.Field {
	return []ent.Field{
		field.Text("text"),
		field.Int("order_index"),
	}
}

func (Questions) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("quiz", Quiz.Type).Ref("questions").Unique().Required(),
		edge.To("answers", Answers.Type),
	}
}

func (Questions) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("order_index"),
		index.Edges("quiz").Fields("order_index").Unique(), // UNIQUE (quiz_id, order_index)
	}
}
