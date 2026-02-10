set GOPROXY=https://goproxy.cn
set GOSUMDB=sum.golang.org
set GOTOOLCHAIN=local

go env GOPROXY GOSUMDB GOTOOLCHAIN
go mod edit -go=1.13
go mod tidy
go test .\internal\argdump\argdump_comp_test.go -v
go mod edit -go=1.18
go mod tidy