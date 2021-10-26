module "master" {
    source="./master"
    aws_ami=var.aws_ami
    aws_user = var.aws_user
    region = var.region
    access_key = var.access_key
    subnets = var.subnets
    key_name = var.key_name
    qa_space = var.qa_space
    resource_name = var.resource_name
    ec2_instance_class = var.ec2_instance_class
    vpc_id = var.vpc_id
    rke2_version = var.rke2_version
    rke2_channel = var.rke2_channel
    server_flags = var.server_flags
    sg_id = var.sg_id
    availability_zone = var.availability_zone
    install_mode = var.install_mode
    no_of_server_nodes_to_join = var.no_of_server_nodes_to_join
    username = var.username
    password = var.password
    node_os = var.node_os
}

module "worker" {
    source="./worker"
    dependency = module.master
    aws_ami = var.aws_ami
    aws_user = var.aws_user
    region = var.region
    access_key = var.access_key
    key_name = var.key_name
    resource_name = var.resource_name
    ec2_instance_class = var.ec2_instance_class
    rke2_version = var.rke2_version
    rke2_channel = var.rke2_channel
    no_of_worker_nodes = var.no_of_worker_nodes
    worker_flags = var.worker_flags
    availability_zone = var.availability_zone
    subnets = var.subnets
    sg_id = var.sg_id
    install_mode = var.install_mode
    username = var.username
    password = var.password
    node_os = var.node_os
}
