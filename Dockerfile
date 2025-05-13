FROM gcr.io/distroless/static-debian11:nonroot
ENTRYPOINT ["/baton-terraform-cloud"]
COPY baton-terraform-cloud /