FROM golang:1.12-stretch as build

RUN apt-get install -y tzdata
RUN curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh

WORKDIR /go/src/github.com/carolynvs/slackoverload
COPY . /go/src/github.com/carolynvs/slackoverload/
RUN CGO_ENABLED=0 go build -a -tags netgo -o bin/slackoverload .

FROM scratch as final

COPY --from=build /go/src/github.com/carolynvs/slackoverload/bin/slackoverload /
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /usr/share/zoneinfo /usr/share/zoneinfo
ENV TZ=America/Chicago

EXPOSE 80
CMD ["/slackoverload"]
