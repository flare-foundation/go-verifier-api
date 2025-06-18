.PHONY: audit lint vulncheck

audit: lint vulncheck

lint:
	go vet ./...
	go tool staticcheck ./...

vulncheck:
	go tool govulncheck ./...