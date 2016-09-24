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
ECS_SERVICE ?= $(PROJECT_NAME)
ECS_TASK_FAMILY ?= $(PROJECT_NAME)
ECS_TASK_DEF_TEMPLATE = ecs-task-def.json
ECS_TASK_DEF_FILE = $(PROJECT_NAME)-task-def.json
ECS_TASK_DEF_REV_URI = anet-ecs-at2.s3.amazonaws.com/revisions/$(ECS_CLUSTER).$(PROJECT_NAME).current.txt
ECS_SERVICE_COUNT ?= 2

MONGODB_URL ?= mongodb://localhost:27017
VASCO_ADDR ?= http://vasco:8081

.PHONY: default test info build
.PHONY: ecr-login ecr-image ecr-task-def ecr-register-task-def ecr-update-service ecr-deploy

default: test

info:
	@echo ORG=$(ORG)
	@echo PROJECT_NAME=$(PROJECT_NAME)
	@echo REVISION=$(REVISION)
	@echo BRANCH=$(BRANCH)
	@echo VERSION=$(VERSION)
	@echo AWS_DEFAULT_REGION=$(AWS_DEFAULT_REGION)
	@echo AWS_ACCOUNT_ID=$(AWS_ACCOUNT_ID)
	@echo ECR_REGISTRY=$(ECR_REGISTRY)
	@echo ECR_REPO=$(ECR_REPO)
	@echo ECR_ENDPOINT=$(ECR_ENDPOINT)
	@echo ECS_CLUSTER=$(ECS_CLUSTER)
	@echo ECS_SERVICE=$(ECS_SERVICE)
	@echo ECS_TASK_FAMILY=$(ECS_TASK_FAMILY)
	@echo ECS_TASK_DEF_TEMPLATE=$(ECS_TASK_DEF_TEMPLATE)
	@echo ECS_TASK_DEF_FILE=$(ECS_TASK_DEF_FILE)
	@echo ECS_TASK_DEF_REV=$(ECS_TASK_DEF_REV)
	@echo ECS_SERVICE_COUNT=$(ECS_SERVICE_COUNT)
	@echo MONGODB_URL=$(MONGODB_URL)
	@echo VASCO_ADDR=$(VASCO_ADDR)

test:
	go test -v -race $(shell glide novendor)

install-deps:
	glide install

build:
	go generate $(shell glide novendor)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o build/$(PROJECT_NAME) -ldflags "-X github.com/AchievementNetwork/go-util/vascoClient.SourceRevision=$(REVISION) -X github.com/AchievementNetwork/go-util/vascoClient.SourceDeployTag=$(VERSION)" .;

ecr-login:
	aws --version
	aws configure set default.region $(AWS_DEFAULT_REGION)
	aws configure set default.output json
	$(shell aws ecr get-login --region $(AWS_DEFAULT_REGION))

ecr-image: build
	docker build -t $(ECR_REPO):$(REVISION) .
	docker tag $(ECR_REPO):$(REVISION) $(ECR_REGISTRY)/$(ECR_REPO):$(REVISION)
	docker push $(ECR_ENDPOINT):$(REVISION)

# Duplicate the template Task Definition and swap placeholders
# for real values as defined in the variables above.
ecs-task-def:
	@cp $(ECS_TASK_DEF_TEMPLATE) $(ECS_TASK_DEF_FILE)
	@sed -i.bak -e s,"<ORG>","$(ORG)",g $(PROJECT_NAME)-task-def.json
	@sed -i.bak -e s,"<PROJECT_NAME>","$(PROJECT_NAME)",g $(PROJECT_NAME)-task-def.json
	@sed -i.bak -e s,"<REVISION>","$(REVISION)",g $(PROJECT_NAME)-task-def.json
	@sed -i.bak -e s,"<ECR_REGISTRY>","$(ECR_REGISTRY)",g $(PROJECT_NAME)-task-def.json
	@sed -i.bak -e s,"<ECS_TASK_FAMILY>","$(ECS_TASK_FAMILY)",g $(PROJECT_NAME)-task-def.json
	@sed -i.bak -e s,"<VASCO_ADDR>","$(VASCO_ADDR)",g $(PROJECT_NAME)-task-def.json
	@sed -i.bak -e s,"<MONGODB_URL>","$(MONGODB_URL)",g $(PROJECT_NAME)-task-def.json
	@rm $(PROJECT_NAME)-task-def.json.bak

ecs-register-task-def: ecs-task-def
	aws ecs register-task-definition --family $(ECS_TASK_FAMILY) --cli-input-json file://$(ECS_TASK_DEF_FILE) --output text

ecs-update-service:
ifndef ECS_TASK_DEF_REV
	$(eval ECS_TASK_DEF_REV = $(shell aws ecs describe-task-definition --task-definition $(PROJECT_NAME) | jq '.taskDefinition.revision'))
endif
	aws ecs update-service \
		--cluster $(ECS_CLUSTER) \
		--service $(ECS_SERVICE) \
		--task-definition $(ECS_TASK_FAMILY):$(ECS_TASK_DEF_REV) \
		--desired-count $(ECS_SERVICE_COUNT) \
		--output text
	
ecs-deploy:
	@ECS_CLUSTER=$(ECS_CLUSTER) \
	ECS_SERVICE=$(ECS_SERVICE) \
	ECS_TASK_FAMILY=$(ECS_TASK_FAMILY) \
	ECS_TASK_DEF_REV=$(ECS_TASK_DEF_REV) \
	REVISION=$(REVISION) \
	$(MAKE) ecs-update-service

