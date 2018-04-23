FROM golang:latest 
RUN mkdir /app 
ADD . /app/ 
WORKDIR /app 
RUN go get -d github.com/Arachnid/etherquery
RUN go get -d github.com/kejace/go-ethereum
RUN go build -o main . 
CMD ["/app/main"]

EXPOSE 8545
EXPOSE 30303
