resource "aws_instance" "master" {
  ami                    = var.aws_ami
  instance_type          = var.ec2_instance_class
  connection {
    type                 = "ssh"
    user                 = var.aws_user
    host                 = self.public_ip
    private_key          = file(var.access_key)
  }
  root_block_device {
    volume_size          = "20"
    volume_type          = "standard"
  }
  subnet_id              = var.subnets
  availability_zone      = var.availability_zone
  vpc_security_group_ids = [var.sg_id]
  key_name               = var.key_name
  tags = {
    Name                 = "${var.resource_name}-server"
  }
  provisioner "file" {
    source      = "install_rke2_master.sh"
    destination = "/tmp/install_rke2_master.sh"
  }
  provisioner "remote-exec" {
    inline = [
      "chmod +x /tmp/install_rke2_master.sh",
      "sudo /tmp/install_rke2_master.sh ${var.node_os} ${aws_route53_record.aws_route53.fqdn}  ${var.rke2_version} ${var.rke2_channel} ${var.install_mode} \"${var.server_flags}\" ${self.public_ip} ${var.username} ${var.password}",
    ]
  }
  provisioner "local-exec" {
    command = "echo ${aws_instance.master.public_ip} >/tmp/${var.resource_name}_master_ip"
  }
  provisioner "local-exec" {
    command = "scp -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i ${var.access_key} ${var.aws_user}@${aws_instance.master.public_ip}:/tmp/nodetoken /tmp/${var.resource_name}_nodetoken"
  }
  provisioner "local-exec" {
    command = "scp -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i ${var.access_key} ${var.aws_user}@${aws_instance.master.public_ip}:/tmp/config /tmp/${var.resource_name}_config"
  }
  provisioner "local-exec" {
    command = "scp -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i ${var.access_key} ${var.aws_user}@${aws_instance.master.public_ip}:/tmp/joinflags /tmp/${var.resource_name}_joinflags"
  }
  provisioner "local-exec" {
    command = "sed s/127.0.0.1/\"${aws_route53_record.aws_route53.fqdn}\"/g /tmp/${var.resource_name}_config >/tmp/${var.resource_name}_kubeconfig"
  }
}

resource "aws_instance" "master2" {
  ami           =  var.aws_ami
  instance_type =  var.ec2_instance_class
  count         =  var.no_of_server_nodes_to_join
  connection {
    type                 = "ssh"
    user                 = var.aws_user
    host                 = self.public_ip
    private_key          = file(var.access_key)
  }
  root_block_device {
    volume_size          = "20"
    volume_type          = "standard"
  }
  subnet_id              = var.subnets
  availability_zone      = var.availability_zone
  vpc_security_group_ids = [var.sg_id]
  key_name               = var.key_name
  depends_on             = [aws_instance.master]
  tags = {
    Name                 = "${var.resource_name}-servers"
  }
  provisioner "file" {
    source      = "join_rke2_master.sh"
    destination = "/tmp/join_rke2_master.sh"
  }
  provisioner "remote-exec" {
    inline = [
      "chmod +x /tmp/join_rke2_master.sh",
      "sudo /tmp/join_rke2_master.sh ${var.node_os} ${aws_route53_record.aws_route53.fqdn} ${aws_instance.master.public_ip} ${local.node_token} ${var.rke2_version} ${var.rke2_channel} ${var.install_mode} \"${var.server_flags}\" ${var.username} ${var.password}",
    ]
  }
}

data "local_file" "token" {
  filename = "/tmp/${var.resource_name}_nodetoken"
  depends_on = ["aws_instance.master"]
}

data "local_file" "kubeconfig" {
  filename = "/tmp/${var.resource_name}_kubeconfig"
  depends_on = ["aws_instance.master"]
}
locals {
  node_token = trimspace("${data.local_file.token.content}")
}

resource "local_file" "master_ips" {
  content     = join("," , aws_instance.master.*.public_ip,aws_instance.master2.*.public_ip)
  filename = "/tmp/${var.resource_name}_master_ips"
}

resource "aws_lb_target_group" "aws_tg_8443" {
  port             = 8443
  protocol         = "TCP"
  vpc_id           = "${var.vpc_id}"
  name             = "${var.resource_name}-8443-tg"
}

resource "aws_lb_target_group" "aws_tg_6443" {
  port             = 6443
  protocol         = "TCP"
  vpc_id           = "${var.vpc_id}"
  name             = "${var.resource_name}-6443-tg"
}

resource "aws_lb_target_group" "aws_tg_9345" {
  port             = 9345
  protocol         = "TCP"
  vpc_id           = "${var.vpc_id}"
  name             = "${var.resource_name}-9345-tg"
}

resource "aws_lb_target_group_attachment" "aws_tg_attachment_8443" {
  target_group_arn = "${aws_lb_target_group.aws_tg_8443.arn}"
  target_id        = "${aws_instance.master.id}"
  port             = 8443
  depends_on       = ["aws_lb_target_group.aws_tg_8443"]
}

resource "aws_lb_target_group_attachment" "aws_tg_attachment_8443_2" {
  target_group_arn = "${aws_lb_target_group.aws_tg_8443.arn}"
  count            = length(aws_instance.master2)
  target_id        = "${aws_instance.master2[count.index].id}"
  port             = 8443
  depends_on       = ["aws_lb_target_group.aws_tg_8443"]
}

resource "aws_lb_target_group_attachment" "aws_tg_attachment_6443" {
  target_group_arn = "${aws_lb_target_group.aws_tg_6443.arn}"
  target_id        = "${aws_instance.master.id}"
  port             = 6443
  depends_on       = ["aws_lb_target_group.aws_tg_6443"]
}

resource "aws_lb_target_group_attachment" "aws_tg_attachment_6443_2" {
  target_group_arn = "${aws_lb_target_group.aws_tg_6443.arn}"
  count            = length(aws_instance.master2)
  target_id        = "${aws_instance.master2[count.index].id}"
  port             = 6443
  depends_on       = ["aws_lb_target_group.aws_tg_6443"]
}

resource "aws_lb_target_group_attachment" "aws_tg_attachment_9345" {
  target_group_arn = "${aws_lb_target_group.aws_tg_9345.arn}"
  target_id        = "${aws_instance.master.id}"
  port             = 9345
  depends_on       = ["aws_lb_target_group.aws_tg_9345"]
}
resource "aws_lb_target_group_attachment" "aws_tg_attachment_9345_2" {
  target_group_arn = "${aws_lb_target_group.aws_tg_9345.arn}"
  count            = length(aws_instance.master2)
  target_id        = "${aws_instance.master2[count.index].id}"
  port             = 9345
  depends_on       = ["aws_lb_target_group.aws_tg_9345"]
}

resource "aws_lb" "aws_nlb" {
  internal           = false
  load_balancer_type = "network"
  subnets            = ["${var.subnets}"]
  name               = "${var.resource_name}-nlb"
}

resource "aws_lb_listener" "aws_nlb_listener_8443" {
  load_balancer_arn = "${aws_lb.aws_nlb.arn}"
  port              = "8443"
  protocol          = "TCP"
  default_action {
    type             = "forward"
    target_group_arn = "${aws_lb_target_group.aws_tg_8443.arn}"
  }
}

resource "aws_lb_listener" "aws_nlb_listener_6443" {
  load_balancer_arn = "${aws_lb.aws_nlb.arn}"
  port              = "6443"
  protocol          = "TCP"
  default_action {
    type             = "forward"
    target_group_arn = "${aws_lb_target_group.aws_tg_6443.arn}"
  }
}

resource "aws_lb_listener" "aws_nlb_listener_9345" {
  load_balancer_arn = "${aws_lb.aws_nlb.arn}"
  port              = "9345"
  protocol          = "TCP"
  default_action {
    type             = "forward"
    target_group_arn = "${aws_lb_target_group.aws_tg_9345.arn}"
  }
}

resource "aws_route53_record" "aws_route53" {
  zone_id            = "${data.aws_route53_zone.selected.zone_id}"
  name               = "${var.resource_name}-route53"
  type               = "CNAME"
  ttl                = "300"
  records            = ["${aws_lb.aws_nlb.dns_name}"]
  depends_on         = ["aws_lb_listener.aws_nlb_listener_6443"]
}

data "aws_route53_zone" "selected" {
  name               = "${var.qa_space}"
  private_zone       = false
}

resource "null_resource" "update_kubeconfig" {
  provisioner "local-exec" {
    command = "sed s/127.0.0.1/\"${aws_route53_record.aws_route53.fqdn}\"/g /tmp/\"${var.resource_name}_config\" >/tmp/${var.resource_name}_kubeconfig"
  }
  depends_on = ["aws_instance.master"]
}

resource "null_resource" "store_fqdn" {
  provisioner "local-exec" {
    command = "echo \"${aws_route53_record.aws_route53.fqdn}\" >/tmp/${var.resource_name}_fixed_reg_addr"
  }
  depends_on = ["aws_instance.master"]
}
