#!/bin/bash

# set AWS_REGION to the region you want to create the bucket in
# set TERRAFORM_STATE_S3_BUCKET_NAME to the name of the bucket you want to create to house your terraform state

# helper function to confirm required variables are set
check_vars()
{
    var_names=("$@")
    for var_name in "${var_names[@]}"; do
        [ -z "${!var_name}" ] && echo "$var_name is unset." && var_unset=true
    done
    [ -n "$var_unset" ] && exit 1
    return 0
}

check_vars AWS_REGION TERRAFORM_STATE_S3_BUCKET_NAME

echo "Creating $TERRAFORM_STATE_S3_BUCKET_NAME in $AWS_REGION"

aws s3api create-bucket --bucket "$TERRAFORM_STATE_S3_BUCKET_NAME" --region "$AWS_REGION"

echo "Enabling bucket versioning on $TERRAFORM_STATE_S3_BUCKET_NAME"

aws s3api put-bucket-versioning --bucket "$TERRAFORM_STATE_S3_BUCKET_NAME" --versioning-configuration Status=Enabled

echo "Bucket created successfully, set the tf_state_bucket variable to $TERRAFORM_STATE_S3_BUCKET_NAME"