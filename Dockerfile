FROM public.ecr.aws/lambda/provided:al2023

RUN dnf install -y \
    tesseract \
    tesseract-langpack-chi_sim \
    tesseract-langpack-eng \
    && dnf clean all

COPY --from=build /bootstrap ${LAMBDA_RUNTIME_DIR}/
COPY aws-config.yml ${LAMBDA_TASK_ROOT}/aws-config.yml

ENV IS_PRODUCTION=true

ENTRYPOINT ["/var/runtime/bootstrap"]