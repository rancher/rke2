# Basic variables
variable "node_os" {}
variable "username" {
  default = "username"
}
variable "password" {
  default = "password"
}
variable "no_of_server_nodes" {}
variable "no_of_worker_nodes" {}
variable "create_lb" {
  description = "Create Network Load Balancer if set to true"
  type = bool
  default = false
}
variable "access_key" {}

# AWS variables
variable "ssh_key" {}
variable "availability_zone" {}
variable "aws_ami" {}
variable "aws_user" {}
variable "ec2_instance_class" {}
variable "volume_size" {}
variable "iam_role" {}
variable "hosted_zone" {}
variable "region" {}
variable "resource_name" {}
variable "sg_id" {}
variable "subnets" {}
variable "vpc_id" {}

# RKE2 variables
variable "rke2_version" {}
variable "install_mode" {
  default = "INSTALL_RKE2_VERSION"
}
variable "install_method" {
  default = null
}
variable "rke2_channel" {
  default = "latest"
}
variable "server_flags" {}
variable "worker_flags" {}
variable "split_roles" {
  description = "When true, server nodes may be a mix of etcd, cp, and worker"
  type = bool
  default = false
}
variable "role_order" {
  description = "Comma separated order of how to bring the nodes up when split roles"
  type = string
  default = "1,2,3,4,5,6"
}
variable "etcd_only_nodes" {
  default = 0
}
variable "etcd_cp_nodes" {
  default = 0
}
variable "etcd_worker_nodes" {
  default = 0
}
variable "cp_only_nodes" {
  default = 0
}
variable "cp_worker_nodes" {
  default = 0
}
variable "optional_files" {
  description = "File location and raw data url separate by commas, with a space for other pairs. E.g. file1,url1 file2,url2"
}