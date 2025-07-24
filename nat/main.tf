##############################################################
# Terraform Module: architect‑nat
# -----------------------------------------------------------
# Highly‑available **Architect NAT** deployment for a *single* AZ.
# It launches **two EC2 instances** (blue & red) wired to two
# dedicated ENIs inside an *Architect subnet*. A secondary
# *floating private IP* plus up to eight Elastic IPs provide
# sub‑second fail‑over without Gateway Load Balancer.
#
# ───────────────────────── MODULE HIGHLIGHTS ─────────────────
# • Builds an **architect subnet** in the caller‑chosen AZ.
# • Creates **eni‑blue** & **eni‑red** with deterministic primary
#   addresses (x.x.x.10 & x.x.x.11) and reserves a **floating
#   private IP** (x.x.x.12) that initially lives on eni‑blue.
# • Allocates or attaches 1‑8 **public IPs (EIPs)** to that
#   floating private IP. `allow_reassociation=true` lets the
#   same association survive a fail‑over.
# • Two Auto Scaling Groups (1 instance each) ensure one EC2 per
#   ENI at all times. Instances carry an IAM role that allows
#   them to run the fail‑over logic (ReplaceRoute, move private
#   IP, re‑associate EIPs).
# • Adds additional configuration knobs (CloudWatch agent, SSM, SSH
#   key, extra SGs, sizing, etc.).
##############################################################