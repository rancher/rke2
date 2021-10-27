resource "aws_instance" "worker" {
  depends_on = [
    var.dependency
  ]
  ami                    = var.aws_ami
  instance_type          = var.ec2_instance_class
  count                  = var.no_of_worker_nodes
  connection {
    type                 = "ssh"
    user                 = var.aws_user
    host                 = self.public_ip
    private_key          = file(var.access_key)
  }
  subnet_id              = var.subnets
  availability_zone      = var.availability_zone
  vpc_security_group_ids = [var.sg_id]
  key_name               = var.key_name
  tags = {
    Name                 = "${var.resource_name}-worker"
  }
  root_block_device {
    volume_size = "20"
    volume_type = "standard"
  }
  provisioner "file" {
    source = "join_rke2_agent.sh"
    destination = "/tmp/join_rke2_agent.sh"
  }
  provisioner "remote-exec" {
    inline = [
      "chmod +x /tmp/join_rke2_agent.sh",
      "sudo /tmp/join_rke2_agent.sh ${var.node_os} ${local.master_fixed_reg_addr} ${local.master_ip} \"${local.node_token}\" ${var.rke2_version} ${var.rke2_channel}    ${var.install_mode} \"${var.worker_flags}\" ${var.username} ${var.password} ",
    ]
  }
}

data "local_file" "master_ip" {
  depends_on = [var.dependency]
  filename = "/tmp/${var.resource_name}_master_ip"
}

locals {
  master_ip = trimspace("${data.local_file.master_ip.content}")
}

data "local_file" "token" {
  depends_on = [var.dependency]
  filename = "/tmp/${var.resource_name}_nodetoken"
}

locals {
  node_token = trimspace("${data.local_file.token.content}")
}

data "local_file" "master_fixed_reg_addr" {
  depends_on = [var.dependency]
  filename = "/tmp/${var.resource_name}_fixed_reg_addr"
}

locals {
  master_fixed_reg_addr = trimspace("${data.local_file.master_fixed_reg_addr.content}")
}

resource "local_file" "worker_ips" {
  content     = join(",", aws_instance.worker.*.public_ip)
  filename = "/tmp/${var.resource_name}_worker_ips"
}

