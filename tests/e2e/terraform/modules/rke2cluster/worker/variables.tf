variable "aws_ami" {}
variable "aws_user" {}
variable "region" {}
variable "subnets" {}
variable "rke2_version" {}
variable "rke2_channel" {}
variable "ec2_instance_class" {}
variable "access_key" {}
variable "key_name" {}
variable "no_of_worker_nodes" {}
variable "worker_flags" {}
variable "resource_name" {}
variable "dependency" {
  type    = any
  default = null
}
variable "install_mode" {}
variable "availability_zone" {}
variable "sg_id" {}
variable "username" {
  default = "username"
}
variable "password" {
  default = "password"
}

variable "node_os" {
  default = "centos"
}
