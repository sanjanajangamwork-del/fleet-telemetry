FROM rust:latest AS builder

WORKDIR /app

COPY Cargo.toml Cargo.lock ./
COPY src ./src

RUN cargo build --release --locked

FROM debian:bookworm-slim

WORKDIR /app

COPY --from=builder /app/target/release/simulator /usr/local/bin/simulator

ENTRYPOINT ["/usr/local/bin/simulator"]