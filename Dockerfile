FROM golang
ADD . /go/src/github.com/tobyjsullivan/event-reader
RUN  go install github.com/tobyjsullivan/event-reader
CMD /go/bin/event-reader
