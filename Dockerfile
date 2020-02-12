FROM golang:1.12-stretch

RUN curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh

WORKDIR /go/src/github.com/carolynvs/slackoverload
COPY . /go/src/github.com/carolynvs/slackoverload/
RUN go build .

EXPOSE 80

CMD ["./slackoverload"]