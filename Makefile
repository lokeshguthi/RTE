rte-go: $(wildcard *.go)
	go build

# package for school deployment:
rte-schule.zip:
	zip rte-schule.zip rte-go inf-schule-readme.md -r webUpload