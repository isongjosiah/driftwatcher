# Configure the AWS Provider
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
  required_version = ">= 1.0"

  # Local backend configuration
  backend "local" {
    path = "terraform.tfstate"
  }
}

# Configure the AWS Provider for LocalStack
provider "aws" {
  region                      = "us-east-1"
  access_key                  = "test"
  secret_key                  = "test"
  skip_credentials_validation = true
  skip_metadata_api_check     = true
  skip_requesting_account_id  = true

  endpoints {
    ec2 = "http://localhost:4566"
  }
}

# No variables - using hardcoded values

# Data source to get a basic AMI for LocalStack
data "aws_ami" "amazon_linux" {
  most_recent = true
  owners      = ["amazon"]

  filter {
    name   = "name"
    values = ["amzn2-ami-hvm-*-x86_64-gp2"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }
}

# Fallback AMI ID for LocalStack if data source fails
locals {
  ami_id = "ami-0abcdef1234567890" # LocalStack default AMI
}

# Create a VPC
resource "aws_vpc" "main" {
  cidr_block           = "10.0.0.0/16"
  enable_dns_hostnames = true
  enable_dns_support   = true

  tags = {
    Name        = "dev-vpc"
    Environment = "dev"
  }
}

# Create an Internet Gateway
resource "aws_internet_gateway" "main" {
  vpc_id = aws_vpc.main.id

  tags = {
    Name        = "dev-igw"
    Environment = "dev"
  }
}

# Create a subnet
resource "aws_subnet" "main" {
  vpc_id                  = aws_vpc.main.id
  cidr_block              = "10.0.1.0/24"
  availability_zone       = data.aws_availability_zones.available.names[0]
  map_public_ip_on_launch = true

  tags = {
    Name        = "dev-subnet"
    Environment = "dev"
  }
}

# Get available AZs
data "aws_availability_zones" "available" {
  state = "available"
}

# Create a route table
resource "aws_route_table" "main" {
  vpc_id = aws_vpc.main.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.main.id
  }

  tags = {
    Name        = "dev-rt"
    Environment = "dev"
  }
}

# Associate the route table with the subnet
resource "aws_route_table_association" "main" {
  subnet_id      = aws_subnet.main.id
  route_table_id = aws_route_table.main.id
}

# Create a security group
resource "aws_security_group" "ec2_sg" {
  name_prefix = "dev-ec2-sg"
  vpc_id      = aws_vpc.main.id

  # SSH access
  ingress {
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  # HTTP access
  ingress {
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  # HTTPS access
  ingress {
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  # All outbound traffic
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name        = "dev-ec2-sg"
    Environment = "dev"
  }
}

# Create the EC2 instance
resource "aws_instance" "main" {
  ami                         = local.ami_id
  instance_type               = "t2.micro"
  vpc_security_group_ids      = [aws_security_group.ec2_sg.id]
  subnet_id                   = aws_subnet.main.id
  associate_public_ip_address = true

  tags = {
    Name        = "dev-ec2-instance"
    Environment = "dev"
  }
}

# Outputs
output "instance_id" {
  description = "ID of the EC2 instance"
  value       = aws_instance.main.id
}

output "instance_public_ip" {
  description = "Public IP address of the EC2 instance"
  value       = aws_instance.main.public_ip
}

output "instance_public_dns" {
  description = "Public DNS name of the EC2 instance"
  value       = aws_instance.main.public_dns
}

output "vpc_id" {
  description = "ID of the VPC"
  value       = aws_vpc.main.id
}

output "security_group_id" {
  description = "ID of the security group"
  value       = aws_security_group.ec2_sg.id
}
