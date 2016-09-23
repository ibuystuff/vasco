#!/usr/bin/env bash

# The following orchestrates the packaging and deployment
# of this application onto an AWS EC2 Container Service
# cluster. The entire script is expected to execute and
# does not take invocation arguments at this time.

# Override these variables using environment variables
# or during script invocation (e.g. ECR_CLUSTER=at2staging ./deploy.sh).
ORG=achievementnetwork # case-sensitive
AWS_DEFAULT_REGION=${AWS_DEFAULT_REGION:-undefined}
AWS_ACCOUNT_ID=${AWS_ACCOUNT_ID:-undefined}
PROJECT_NAME=${PROJECT_NAME:-$CIRCLE_PROJECT_REPONAME}
ECR_REPO=${ECR_REPO:-$ORG/$PROJECT_NAME}
ECR_TAG=${ECR_TAG:-$CIRCLE_SHA1}
ECR_REGISTRY=$AWS_ACCOUNT_ID.dkr.ecr.us-east-1.amazonaws.com
ECR_ENDPOINT=$ECR_REGISTRY/$ECR_REPO
ECS_CLUSTER=${ECS_CLUSTER:-undefined}
ECS_TASK_FAMILY=${ECS_TASK_FAMILY:-$PROJECT_NAME}
ECS_SERVICE=${ECS_SERVICE:-$PROJECT_NAME}
MONGODB_URL=${MONGODB_URL:-mongodb://localhost:27017}
VASCO_ADDR=${VASCO_ADDR:-http://vasco:8081}

# Prints out the variables that will drive the deploy.
print_config(){
	echo AWS_DEFAULT_REGION=$AWS_DEFAULT_REGION
	echo AWS_ACCOUNT_ID=$AWS_ACCOUNT_ID
	echo PROJECT_NAME=$PROJECT_NAME
	echo ECR_REPO=$ECR_REPO
	echo ECR_TAG=$ECR_TAG
	echo ECR_ENDPOINT=$ECR_ENDPOINT
	echo ECS_CLUSTER=$ECS_CLUSTER
	echo ECS_TASK_FAMILY=$ECS_TASK_FAMILY
	echo ECS_SERVICE=$ECS_SERVICE
	echo MONGODB_URL=$MONGODB_URL
	echo VASCO_ADDR=$VASCO_ADDR
	echo
}

# Assumes the aws cli tool is installed in the environment.
configure_aws_cli(){
	aws --version
	aws configure set default.region $AWS_DEFAULT_REGION
	aws configure set default.output json
	eval $(aws ecr get-login --region $AWS_DEFAULT_REGION)
}

# Builds and pushes our docker image to AWS ECR.
build_image(){
	make build
	docker build -t $ECR_REPO:$ECR_TAG .
	docker tag $ECR_REPO:$ECR_TAG $ECR_REGISTRY/$ECR_REPO:$ECR_TAG
	docker push $ECR_ENDPOINT:$ECR_TAG
}

# Generate the AWS ECS Task Definition for this service.
generate_task_def(){
	# Duplicate the template Task Definition and swap placeholders
	# for real values as defined in the variables above.
	cp ecs-task-def.json task-def.json

	# Warning: The following sed commands do not work on MacOS
	sed -i -e s,"<ORG>","$ORG",g task-def.json
	sed -i -e s,"<PROJECT_NAME>","$PROJECT_NAME",g task-def.json
	sed -i -e s,"<VASCO_ADDR>","$VASCO_ADDR",g task-def.json
	sed -i -e s,"<MONGODB_URL>","$MONGODB_URL",g task-def.json
	sed -i -e s,"<ECR_REGISTRY>","$ECR_REGISTRY",g task-def.json
	sed -i -e s,"<ECS_TASK_FAMILY>","$ECS_TASK_FAMILY",g task-def.json
}

# AWS ECS needs to be aware of the latest revision to the Task Definition.
register_task_def() {
	if revision=$(aws ecs register-task-definition --family $ECS_TASK_FAMILY --cli-input-json file://task-def.json); then
		echo "Task Definition Registered: $ECS_TASK_FAMILY"
	else
		echo "Failed to register task definition"
		return 1
	fi
}

# AWS ECS needs to relauch the service that makes use of this Task Definition after each update.
update_service() {
	if result=$(aws ecs update-service --cluster $ECS_CLUSTER --service $ECS_SERVICE --task-definition $ECS_TASK_FAMILY); then
		echo "Service Updated: $ECS_SERVICE"
	else
		echo "Error updating service."
		return 1
	fi
}

print_config
configure_aws_cli
build_image
generate_task_def
register_task_def
update_service

