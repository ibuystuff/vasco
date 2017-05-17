REVISION ?= $(shell git rev-parse --short HEAD)
BRANCH ?= $(shell git branch |sort |tail -1 |cut -c 3-)
VERSION ?= "Branch:$(BRANCH)"
DEPLOY_TAG ?= $(VERSION)
DEPLOYTYPE ?= devel
CONFIGVERSION ?= None

PROJECT ?= $(shell basename $(CURDIR))
VASCO_PROXY ?= 8080
VASCO_REGISTRY ?= 8081
VASCO_STATUS ?= 8082
REDIS_ADDR ?= localhost:6379
MINPORT ?= 8100
MAXPORT ?= 9900
EXPECTED_SERVICES ?= assess item passage pdf sas staticserver stdmirror stdtag user assessapi learnm doctag
STATUS_TIME ?= 60
DISCOVERY_EXPIRATION ?= 3600
STATIC_PATH ?= /static
USE_SWAGGER ?= false
IAM_TOKEN_SIGNING_KEY ?= unspecified
IAM_SSO_COOKIE ?= iam-sso-dev

ECS_CLUSTER ?= default
ECS_SERVICE_COUNT ?= 1
ECS_SERVICE_MAX_PERCENT ?= 200
ECS_SERVICE_MIN_HEALTHY_PERCENT ?= 100
ECS_TASK_MEMORY ?= 100

ENVARS = REVISION|$(REVISION),DEPLOYTAG|$(DEPLOY_TAG),DEPLOYTYPE|$(DEPLOYTYPE),CONFIGVERSION|$(CONFIGVERSION),VASCO_PROXY|$(VASCO_PROXY),VASCO_REGISTRY|$(VASCO_REGISTRY),VASCO_STATUS|$(VASCO_STATUS),REDIS_ADDR|$(REDIS_ADDR),MINPORT|$(MINPORT),MAXPORT|$(MAXPORT),EXPECTED_SERVICES|$(EXPECTED_SERVICES),STATUS_TIME|$(STATUS_TIME),DISCOVERY_EXPIRATION|$(DISCOVERY_EXPIRATION),STATIC_PATH|$(STATIC_PATH),USE_SWAGGER|$(USE_SWAGGER),IAM_TOKEN_SIGNING_KEY|$(IAM_TOKEN_SIGNING_KEY),IAM_SSO_COOKIE|$(IAM_SSO_COOKIE)

.PHONY: default test build install-deps
.PHONY: ecr-image ecs-register-task
.PHONY: ecs-create-service ecs-update-service
.PHONY: ecs-deploy ecs-update-service

default: test

test:
	go test -v -race $(shell glide novendor)

install-deps:
	glide install

build:
	go generate $(shell glide novendor)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o build/$(PROJECT) .

ecr-image: build
	ecs new-image $(PROJECT) $(REVISION) $(ECS_CLUSTER)

ecs-create-service: ecs-register-task
	ecs create-service $(PROJECT) $(REVISION) $(ECS_CLUSTER) \
		--ecs-service-count=$(ECS_SERVICE_COUNT) \
		--ecs-service-max-percent=$(ECS_SERVICE_MAX_PERCENT) \
		--ecs-service-min-healthy-percent=$(ECS_SERVICE_MIN_HEALTHY_PERCENT)

ecs-update-service: ecs-register-task
	ecs update-service $(PROJECT) $(REVISION) $(ECS_CLUSTER) \
		--ecs-service-count=$(ECS_SERVICE_COUNT) \
		--ecs-service-max-percent=$(ECS_SERVICE_MAX_PERCENT) \
		--ecs-service-min-healthy-percent=$(ECS_SERVICE_MIN_HEALTHY_PERCENT)

ecs-register-task:
	ecs register-task $(PROJECT) $(REVISION) $(ECS_CLUSTER) \
		--branch=$(BRANCH) --version=$(DEPLOY_TAG) \
		--port-mappings="$(VASCO_PROXY):$(VASCO_PROXY),$(VASCO_REGISTRY):$(VASCO_REGISTRY),$(VASCO_STATUS):$(VASCO_STATUS)" \
		--ecs-task-memory=$(ECS_TASK_MEMORY) \
		--envars="$(ENVARS)"

ecs-first-deploy: ecr-image ecs-create-service
	@echo "First Deploy Complete"

ecs-deploy: ecr-image ecs-update-service
	@echo "Deploy Complete"
