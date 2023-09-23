#!/bin/bash

export AWS_ACCESS_KEY_ID=fake
export AWS_SECRET_ACCESS_KEY=fake
export AWS_DEFAULT_REGION=us-east-1
export AWS_PAGER=

ENDPOINT=$1
: ${ENDPOINT:=http://dynamodb:8000}

aws dynamodb create-table \
    --endpoint-url $ENDPOINT \
    --billing-mode PAY_PER_REQUEST \
    --table-name label-mutex \
    --attribute-definitions AttributeName=id,AttributeType=S AttributeName=name,AttributeType=S \
    --key-schema AttributeName=id,KeyType=HASH AttributeName=name,KeyType=RANGE
aws dynamodb update-time-to-live \
    --endpoint-url $ENDPOINT \
    --table-name label-mutex \
    --time-to-live-specification Enabled=true,AttributeName=expires
