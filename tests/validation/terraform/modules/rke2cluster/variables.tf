variable "aws_ami" {}
variable "aws_user" {}
variable "region" {}
variable "access_key" {}
variable "key_name" {}
variable "vpc_id" {}
variable "subnets" {}
variable "qa_space" {}
variable "resource_name" {}
variable "ec2_instance_class" {}
variable "rke2_version" {}
variable "availability_zone" {}
variable "sg_id" {}
variable "server_flags" {}
variable "worker_flags" {}
variable "no_of_worker_nodes" {}
variable "no_of_server_nodes_to_join" {}
variable "install_mode" {}
variable "username" {
  default = "username"
}
variable "password" {
  default = "password"
}
variable "ctype" {
  default = "centos"
}
