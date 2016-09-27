ORG = achievementnetwork
PROJECT_NAME = $(shell basename $(CURDIR))

REVISION ?= $(shell git rev-parse --short HEAD)
BRANCH ?= $(shell git branch |sort |tail -1 |cut -c 3-)
VERSION ?= "Branch:$(BRANCH)"

AWS_DEFAULT_REGION ?= us-east-1
AWS_ACCOUNT_ID ?= undefined

ECR_REGISTRY = $(AWS_ACCOUNT_ID).dkr.ecr.us-east-1.amazonaws.com
ECR_REPO ?= $(ORG)/$(PROJECT_NAME)
ECR_ENDPOINT=$(ECR_REGISTRY)/$(ECR_REPO)

ECS_CLUSTER ?= default
ECS_TASK_FAMILY ?= $(PROJECT_NAME)
ECS_TASK_DEF_TEMPLATE = ecs-task-def.json
ECS_TASK_DEF_FILE = $(PROJECT_NAME)-task-def.json
ECS_SERVICE ?= $(PROJECT_NAME)
ECS_SERVICE_COUNT ?= 2
ECS_SERVICE_MAX_PERCENT ?= 100
ECS_SERVICE_MIN_HEALTHY_PERCENT ?= 50
ECS_SERVICE_DEF_TEMPLATE = ecs-service-def.json
ECS_SERVICE_DEF_FILE = $(PROJECT_NAME)-service-def.json

AWS_LOG_GROUP = ecs-$(ECS_CLUSTER)
AWS_LOG_REGION = $(AWS_DEFAULT_REGION)
AWS_LOG_STREAM_PREFIX = $(ECS_SERVICE)

# TODO: Store information of deployed revisions in S3
#ECS_TASK_DEF_REV_URI = anet-ecs-at2.s3.amazonaws.com/revisions/$(ECS_CLUSTER).$(PROJECT_NAME).current.txt

REDIS_ADDR ?= localhost:6379

.PHONY: default test info build
.PHONY: ecr-login ecr-image
.PHONY: ecs-task-def ecs-register-task-def
.PHONY: ecs-create-service ecs-update-service
.PHONY: ecs-deploy

default: test

info:
	@echo ORG=$(ORG)
	@echo PROJECT_NAME=$(PROJECT_NAME)
	@echo REVISION=$(REVISION)
	@echo BRANCH=$(BRANCH)
	@echo VERSION=$(VERSION)
	@echo AWS_DEFAULT_REGION=$(AWS_DEFAULT_REGION)
	@echo AWS_ACCOUNT_ID=$(AWS_ACCOUNT_ID)
	@echo AWS_LOG_GROUP=$(AWS_LOG_GROUP)
	@echo AWS_LOG_REGION=$(AWS_LOG_REGION)
	@echo AWS_LOG_STREAM_PREFIX=$(AWS_LOG_STREAM_PREFIX)
	@echo ECR_REGISTRY=$(ECR_REGISTRY)
	@echo ECR_REPO=$(ECR_REPO)
	@echo ECR_ENDPOINT=$(ECR_ENDPOINT)
	@echo ECS_CLUSTER=$(ECS_CLUSTER)
	@echo ECS_TASK_FAMILY=$(ECS_TASK_FAMILY)
	@echo ECS_TASK_DEF_TEMPLATE=$(ECS_TASK_DEF_TEMPLATE)
	@echo ECS_TASK_DEF_FILE=$(ECS_TASK_DEF_FILE)
	@echo ECS_TASK_DEF_REV=$(ECS_TASK_DEF_REV)
	@echo ECS_SERVICE=$(ECS_SERVICE)
	@echo ECS_SERVICE_COUNT=$(ECS_SERVICE_COUNT)
	@echo ECS_SERVICE_MAX_PERCENT=$(ECS_SERVICE_MAX_PERCENT)
	@echo ECS_SERVICE_MIN_HEALTHY_PERCENT=$(ECS_SERVICE_MIN_HEALTHY_PERCENT)
	@echo ECS_SERVICE_DEF_TEMPLATE=$(ECS_SERVICE_DEF_TEMPLATE)
	@echo ECS_SERVICE_DEF_FILE=$(ECS_SERVICE_DEF_FILE)
	@echo REDIS_ADDR=$(REDIS_ADDR)

test:
	go test -v -race $(shell glide novendor)

install-deps:
	glide install

build:
	go generate $(shell glide novendor)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o build/$(PROJECT_NAME) -ldflags \
		"-X github.com/AchievementNetwork/go-util/vascoClient.SourceRevision=$(REVISION) \
		-X github.com/AchievementNetwork/go-util/vascoClient.SourceDeployTag=$(VERSION)" .;

ecr-login:
	aws --version
	aws configure set default.region $(AWS_DEFAULT_REGION)
	aws configure set default.output json
	$(shell aws ecr get-login --region $(AWS_DEFAULT_REGION))

ecr-image: build
	docker build -t $(ECR_REPO):$(REVISION) .
	docker tag $(ECR_REPO):$(REVISION) $(ECR_REGISTRY)/$(ECR_REPO):$(REVISION)
	docker push $(ECR_ENDPOINT):$(REVISION)

ecs-task-def:
	@cp $(ECS_TASK_DEF_TEMPLATE) $(ECS_TASK_DEF_FILE)
	@sed -i.bak -e s,"<ORG>","$(ORG)",g $(PROJECT_NAME)-task-def.json
	@sed -i.bak -e s,"<PROJECT_NAME>","$(PROJECT_NAME)",g $(PROJECT_NAME)-task-def.json
	@sed -i.bak -e s,"<REVISION>","$(REVISION)",g $(PROJECT_NAME)-task-def.json
	@sed -i.bak -e s,"<ECR_REGISTRY>","$(ECR_REGISTRY)",g $(PROJECT_NAME)-task-def.json
	@sed -i.bak -e s,"<ECS_TASK_FAMILY>","$(ECS_TASK_FAMILY)",g $(PROJECT_NAME)-task-def.json
	@sed -i.bak -e s,"<AWS_LOG_GROUP>","$(AWS_LOG_GROUP)",g $(PROJECT_NAME)-task-def.json
	@sed -i.bak -e s,"<AWS_LOG_REGION>","$(AWS_LOG_REGION)",g $(PROJECT_NAME)-task-def.json
	@sed -i.bak -e s,"<AWS_LOG_STREAM_PREFIX>","$(AWS_LOG_STREAM_PREFIX)",g $(PROJECT_NAME)-task-def.json
	@sed -i.bak -e s,"<REDIS_ADDR>","$(REDIS_ADDR)",g $(PROJECT_NAME)-task-def.json
	@rm $(PROJECT_NAME)-task-def.json.bak

ecs-register-task-def: ecs-task-def
	aws ecs register-task-definition --family $(ECS_TASK_FAMILY) --cli-input-json file://$(ECS_TASK_DEF_FILE) --output text
	@rm $(PROJECT_NAME)-task-def.json
	aws ecs describe-task-definition --task-definition $(PROJECT_NAME) --output text

ecs-service-def:
	@cp $(ECS_SERVICE_DEF_TEMPLATE) $(ECS_SERVICE_DEF_FILE)
	@sed -i.bak -e s,"<ECS_CLUSTER>","$(ECS_CLUSTER)",g $(PROJECT_NAME)-service-def.json
	@sed -i.bak -e s,"<ECS_SERVICE>","$(ECS_SERVICE)",g $(PROJECT_NAME)-service-def.json
	@sed -i.bak -e s,"<ECS_TASK_FAMILY>","$(ECS_TASK_FAMILY)",g $(PROJECT_NAME)-service-def.json
	@sed -i.bak -e s,"<ECS_SERVICE_COUNT>","$(ECS_SERVICE_COUNT)",g $(PROJECT_NAME)-service-def.json
	@sed -i.bak -e s,"<ECS_SERVICE_MAX_PERCENT>","$(ECS_SERVICE_MAX_PERCENT)",g $(PROJECT_NAME)-service-def.json
	@sed -i.bak -e s,"<ECS_SERVICE_MIN_HEALTHY_PERCENT>","$(ECS_SERVICE_MIN_HEALTHY_PERCENT)",g $(PROJECT_NAME)-service-def.json
	@rm $(PROJECT_NAME)-service-def.json.bak

ecs-create-service: ecs-service-def
	aws ecs create-service --cluster $(ECS_CLUSTER) --cli-input-json file://$(ECS_SERVICE_DEF_FILE) --output text
	@rm $(PROJECT_NAME)-service-def.json

ecs-update-service:
ifndef ECS_TASK_DEF_REV
	$(eval ECS_TASK_DEF_REV = $(shell aws ecs describe-task-definition --task-definition $(PROJECT_NAME) | jq '.taskDefinition.revision'))
endif
	$(MAKE) info
	aws ecs update-service \
		--cluster $(ECS_CLUSTER) \
		--service $(ECS_SERVICE) \
		--task-definition $(ECS_TASK_FAMILY):$(ECS_TASK_DEF_REV) \
		--desired-count $(ECS_SERVICE_COUNT) \
		--deployment-configuration maximumPercent=$(ECS_SERVICE_MAX_PERCENT),minimumHealthyPercent=$(ECS_SERVICE_MIN_HEALTHY_PERCENT) \
		--output text
	
# Uses default task and service configuration params that will
# typically be overriden for non-development environments.
ecs-deploy:
	@ECS_CLUSTER=$(ECS_CLUSTER) \
	ECS_TASK_FAMILY=$(ECS_TASK_FAMILY) \
	ECS_TASK_DEF_REV=$(ECS_TASK_DEF_REV) \
	ECS_SERVICE=$(ECS_SERVICE) \
	ECS_SERVICE_COUNT=$(ECS_SERVICE_COUNT) \
	ECS_SERVICE_MAX_PERCENT=$(ECS_SERVICE_MAX_PERCENT) \
	ECS_SERVICE_MIN_HEALTHY_PERCENT=$(ECS_SERVICE_MIN_HEALTHY_PERCENT) \
	REVISION=$(REVISION) \
	$(MAKE) ecs-update-service

