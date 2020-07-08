resource "aws_iam_instance_profile" "nomad_server" {
  name_prefix = var.stack_name
  role        = aws_iam_role.nomad_server.name
}

resource "aws_iam_role" "nomad_server" {
  name_prefix        = var.stack_name
  assume_role_policy = data.aws_iam_policy_document.nomad_server_assume.json
}

resource "aws_iam_role_policy" "nomad_server" {
  name   = "nomad-server"
  role   = aws_iam_role.nomad_server.id
  policy = data.aws_iam_policy_document.nomad_server.json
}

data "aws_iam_policy_document" "nomad_server_assume" {
  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["ec2.amazonaws.com"]
    }
  }
}

data "aws_iam_policy_document" "nomad_server" {
  statement {
    effect = "Allow"

    actions = [
      "ec2:DescribeInstances",
      "ec2:DescribeTags",
      "autoscaling:DescribeAutoScalingGroups",
    ]

    resources = ["*"]
  }
}

resource "aws_iam_instance_profile" "nomad_client" {
  name_prefix = var.stack_name
  role        = aws_iam_role.nomad_client.name
}

resource "aws_iam_role" "nomad_client" {
  name_prefix        = var.stack_name
  assume_role_policy = data.aws_iam_policy_document.nomad_client_assume.json
}

resource "aws_iam_role_policy" "nomad_client" {
  name   = "noamd-client"
  role   = aws_iam_role.nomad_client.id
  policy = data.aws_iam_policy_document.nomad_client.json
}

data "aws_iam_policy_document" "nomad_client_assume" {
  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["ec2.amazonaws.com"]
    }
  }
}

data "aws_iam_policy_document" "nomad_client" {
  statement {
    effect = "Allow"

    actions = [
      "autoscaling:CreateOrUpdateTags",
      "autoscaling:DescribeScalingActivities",
      "autoscaling:DescribeAutoScalingGroups",
      "autoscaling:DetachInstances",
      "autoscaling:UpdateAutoScalingGroup",
      "ec2:TerminateInstances",
      "ec2:DescribeInstances",
      "ec2:DescribeInstanceStatus",
      "ec2:DescribeTags",
    ]

    resources = ["*"]
  }
}
