DB_DSN := root:wkr1835484520@tcp(localhost:3306)/auth_service?parseTime=true&loc=Local

.PHONY: migrate
migrate:
	go run github.com/pressly/goose/v3/cmd/goose@latest \-dir db/migrations mysql "$(DB_DSN)" up
