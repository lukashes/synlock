docker-compose:
	docker-compose up -d

test: docker-compose
	go test -v -count=1 ./...

lint:
	golangci-lint run --enable-all --disable gochecknoglobals --disable staticcheck

clean:
	docker-compose down