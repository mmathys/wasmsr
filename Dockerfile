FROM golang

WORKDIR /app

COPY go.* ./
RUN go mod download

COPY . .

RUN mkdir testdata
RUN cp examples/snapshot/testdata/transcoding.wasm testdata/
RUN cp examples/snapshot/video1s.mpeg .

RUN go build examples/snapshot/snapshot.go 
EXPOSE 50051
EXPOSE 50052