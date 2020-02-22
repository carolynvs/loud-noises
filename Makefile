VERSION=v0.0.3

build:
	docker build -t carolynvs/slackoverload:${VERSION} .
	cd bundle && porter build

deploy: build
	docker push carolynvs/slackoverload:${VERSION}
	cd bundle && porter upgrade -c slackoverload

logs:
	cd bundle && porter invoke --action logs -c slackoverload