test:
	go test -v .

bench:
	go test -bench .

examples:
	go run ./example/common/common.go
	go run ./example/nginx/nginx.go
	cat ./example/access.log | go run ./example/nginx/nginx.go --log=-
