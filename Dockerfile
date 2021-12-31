FROM alpine
COPY translate index.html /
CMD ["/translate"]