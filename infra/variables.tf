variable "tf_state_bucket" {
  description = "The name of the S3 bucket for storing the Terraform state"
  type        = string
}

variable "tf_state_key" {
  description = "The path to the Terraform state file inside the S3 bucket"
  type        = string
  default     = "terraform/state/terraform.tfstate"
}

variable "tf_state_region" {
  description = "The AWS region where the terraform state's S3 bucket is located"
  type        = string
  default     = "us-east-1"
}