FROM alpine/git:1.0.4 as clone
RUN mkdir /code
RUN git clone --depth 1 https://github.com/hakur/k8s-tty-device /code



FROM golang:1.10.3-alpine as build
RUN mkdir -p /gopath/src/github.com/hakur
ENV GOPATH=/gopath
COPY --from=clone /code/k8s-tty-device /gopath/src/github.com/hakur/
RUN go build -ldflags "-s -w" -o /gopath/k8s-tty-device /gopath/src/github.com/hakur/k8s-tty-device



FROM alpine:3.7
COPY --from=build /gopath/k8s-tty-device /bin/
RUN chmod +x /bin/k8s-tty-device
CMD ["/bin/k8s-tty-device"]
