resource "aws_launch_template" "nomad_client" {
  name_prefix            = "nomad-client"
  image_id               = var.ami
  instance_type          = var.client_instance_type
  key_name               = var.key_name
  vpc_security_group_ids = [aws_security_group.primary.id]
  user_data              = base64encode(data.template_file.user_data_client.rendered)

  iam_instance_profile {
    name = aws_iam_instance_profile.nomad_client.name
  }

  tag_specifications {
    resource_type = "instance"
    tags = {
      Name           = "${var.stack_name}-client"
      ConsulAutoJoin = "auto-join"
    }
  }

  block_device_mappings {
    device_name = "/dev/xvdd"
    ebs {
      volume_type           = "gp2"
      volume_size           = "50"
      delete_on_termination = "true"
    }
  }
}

resource "aws_autoscaling_group" "nomad_client" {
  name               = "${var.stack_name}-nomad_client"
  availability_zones = var.availability_zones
  desired_capacity   = var.client_count
  min_size           = 0
  max_size           = 10
  depends_on         = [aws_instance.nomad_server]
  load_balancers     = [aws_elb.nomad_client.name]

  launch_template {
    id      = aws_launch_template.nomad_client.id
    version = "$Latest"
  }

  tag {
    key                 = "OwnerName"
    value               = var.owner_name
    propagate_at_launch = true
  }
  tag {
    key                 = "OwnerEmail"
    value               = var.owner_email
    propagate_at_launch = true
  }
}