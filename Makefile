docker-compose:
	docker-compose pull
	docker-compose up -d && sleep 5
	#docker-compose logs

test: docker-compose
	go test -v -count=1 ./...

lint:
	golangci-lint run --enable-all --disable gochecknoglobals --disable staticcheck

clean:
	docker-compose down