data "external_schema" "ent" {
  program = [
    "go",
    "run",
    "-mod=mod",
    "entgo.io/ent/cmd/ent",
    "describe",
    "./internal/store/ent/schema",
  ]
}

env "local" {
  src = "ent://internal/store/ent/schema"
  dev = "postgresql://3yvti:local@127.0.0.1:9500/quiz_db?sslmode=disable&search_path=atlas_dev"
  url = "postgresql://3yvti:local@127.0.0.1:9500/quiz_db?sslmode=disable"
  migration {
    dir = "file://internal/store/migrations"
  }
}