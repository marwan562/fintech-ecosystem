terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.23"
    }
  }
  required_version = ">= 1.5.0"
}

provider "aws" {
  region = var.region
  
  default_tags {
    tags = {
      Environment = var.environment
      Project     = "sapliy-cloud"
      ManagedBy   = "terraform"
    }
  }
}

locals {
  name = "sapliy-${var.environment}"
}

module "vpc" {
  source = "terraform-aws-modules/vpc/aws"
  version = "5.1.2"

  name = "${local.name}-vpc"
  cidr = var.vpc_cidr

  azs             = ["${var.region}a", "${var.region}b", "${var.region}c"]
  private_subnets = [for k, v in module.vpc.azs : cidrsubnet(var.vpc_cidr, 8, k)]
  public_subnets  = [for k, v in module.vpc.azs : cidrsubnet(var.vpc_cidr, 8, k + 4)]
  database_subnets = [for k, v in module.vpc.azs : cidrsubnet(var.vpc_cidr, 8, k + 8)]

  enable_nat_gateway = true
  single_nat_gateway = true # Cost saving for non-prod
  enable_vpn_gateway = false
}

module "eks" {
  source  = "terraform-aws-modules/eks/aws"
  version = "19.19.0"

  cluster_name    = "${local.name}-cluster"
  cluster_version = "1.28"

  vpc_id     = module.vpc.vpc_id
  subnet_ids = module.vpc.private_subnets

  cluster_endpoint_public_access = true

  eks_managed_node_group_defaults = {
    ami_type = "AL2_x86_64"
  }

  eks_managed_node_groups = {
    general = {
      name = "general-wg"
      instance_types = ["t3.large"]
      min_size     = 1
      max_size     = 3
      desired_size = 2
    }
  }
}

module "rds" {
  source  = "terraform-aws-modules/rds/aws"
  version = "6.2.0"

  identifier = "${local.name}-db"

  engine               = "postgres"
  engine_version       = "15.4"
  family               = "postgres15"
  major_engine_version = "15"
  instance_class       = var.db_instance_class

  allocated_storage     = 20
  max_allocated_storage = 100

  db_name  = "microservices"
  username = "sapliy_admin"
  port     = 5432

  manage_master_user_password = true # Auto-create secret in Secrets Manager

  db_subnet_group_name   = module.vpc.database_subnet_group
  vpc_security_group_ids = [module.security_group_db.security_group_id]

  maintenance_window = "Mon:00:00-Mon:03:00"
  backup_window      = "03:00-06:00"
  
  # Encryption
  storage_encrypted = true
  deletion_protection = var.environment == "prod"
}

module "elasticache" {
  source  = "cloudposse/elasticache-redis/aws"
  version = "0.52.0"

  name           = "${local.name}-redis"
  vpc_id         = module.vpc.vpc_id
  subnets        = module.vpc.private_subnets
  cluster_size   = 2
  instance_type  = "cache.t4g.small"
  engine_version = "7.0"
  
  transit_encryption_enabled = true
  at_rest_encryption_enabled = true
}

# Simple Security Group for DB Access from EKS
module "security_group_db" {
  source  = "terraform-aws-modules/security-group/aws"
  version = "5.1.0"

  name        = "${local.name}-db-sg"
  description = "PostgreSQL security group"
  vpc_id      = module.vpc.vpc_id

  ingress_with_cidr_blocks = [
    {
      from_port   = 5432
      to_port     = 5432
      protocol    = "tcp"
      description = "PostgreSQL access from VPC"
      cidr_blocks = module.vpc.vpc_cidr_block
    },
  ]
}
