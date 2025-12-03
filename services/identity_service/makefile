DB_DSN := root:imdev@tcp(localhost:3306)/identity_service?parseTime=true&loc=Local

.PHONY: migrate
migrate:
	go run github.com/pressly/goose/v3/cmd/goose@latest \-dir db/migrations mysql "$(DB_DSN)" up
