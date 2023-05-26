# Server Nodes
module "master" {
  source = "./master"

  # Basic variables
  node_os            = var.node_os
  no_of_server_nodes = var.no_of_server_nodes
  create_lb          = var.create_lb
  username           = var.username
  password           = var.password
  all_role_nodes     = var.no_of_server_nodes
  etcd_only_nodes    = var.etcd_only_nodes
  etcd_cp_nodes      = var.etcd_cp_nodes
  etcd_worker_nodes  = var.etcd_worker_nodes
  cp_only_nodes      = var.cp_only_nodes
  cp_worker_nodes    = var.cp_worker_nodes
  optional_files     = var.optional_files

  # AWS variables
  access_key         = var.access_key
  ssh_key            = var.ssh_key
  availability_zone  = var.availability_zone
  aws_ami            = var.aws_ami
  aws_user           = var.aws_user
  ec2_instance_class = var.ec2_instance_class
  volume_size        = var.volume_size
  iam_role           = var.iam_role
  hosted_zone        = var.hosted_zone
  region             = var.region
  resource_name      = var.resource_name
  sg_id              = var.sg_id
  subnets            = var.subnets
  vpc_id             = var.vpc_id

  # RKE2 variables
  rke2_version   = var.rke2_version
  install_mode   = var.install_mode
  install_method = var.install_method
  rke2_channel   = var.rke2_channel
  server_flags   = var.server_flags
  split_roles    = var.split_roles
  role_order     = var.role_order
}

# Agent Nodes
module "worker" {
  source     = "./worker"
  dependency = module.master

  # Basic variables
  node_os            = var.node_os
  no_of_worker_nodes = var.no_of_worker_nodes
  username           = var.username
  password           = var.password

  # AWS variables
  access_key         = var.access_key
  ssh_key            = var.ssh_key
  availability_zone  = var.availability_zone
  aws_ami            = var.aws_ami
  aws_user           = var.aws_user
  ec2_instance_class = var.ec2_instance_class
  volume_size        = var.volume_size
  iam_role           = var.iam_role
  region             = var.region
  resource_name      = var.resource_name
  sg_id              = var.sg_id
  subnets            = var.subnets
  vpc_id             = var.vpc_id

  # RKE2 variables
  rke2_version   = var.rke2_version
  install_mode   = var.install_mode
  install_method = var.install_method
  rke2_channel   = var.rke2_channel
  worker_flags   = var.worker_flags
}