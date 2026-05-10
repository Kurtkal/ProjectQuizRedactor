package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type Answers struct {
	ent.Schema
}

func (Answers) Fields() []ent.Field {
	return []ent.Field{
		field.Text("text"),
		field.Bool("is_correct").Default(false),
		field.Int("order_index"),
	}
}

func (Answers) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("question", Questions.Type).Ref("answers").Unique().Required(),
	}
}

func (Answers) Indexes() []ent.Index {
	return []ent.Index{
		index.Edges("question").Fields("order_index").Unique(),
	}
}
