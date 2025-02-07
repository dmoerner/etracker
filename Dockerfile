FROM golang:1.23 AS builder-go
WORKDIR /app
COPY ./go.mod ./go.sum ./
RUN go mod download && go mod verify
COPY . .
RUN go build -v -o / ./...

FROM node:22 AS builder-node
WORKDIR /app
COPY frontend/package*.json ./
RUN npm install
COPY frontend .
RUN npm run build

FROM ubuntu
COPY --from=builder-go /etracker /etracker
RUN mkdir -p /frontend
COPY --from=builder-node /app/dist ./frontend/dist

CMD ["/etracker"]
