##############################################################
# Terraform Module: architect‑nat
# -----------------------------------------------------------
# Highly‑available **Architect NAT** deployment for a *single* AZ.
# It launches **two EC2 instances** (blue & red) wired to two
# dedicated ENIs inside an *Architect subnet*. Up to eight
# secondary *floating private IPs* (and their associated Elastic
# IPs) provide sub‑second fail‑over without Gateway Load Balancer.
#
# ───────────────────────── MODULE HIGHLIGHTS ─────────────────
# • Builds an **architect subnet** in the caller‑chosen AZ.
# • Creates **eni‑blue** & **eni‑red** with deterministic management
#   addresses (x.x.x.10 & x.x.x.11) and reserves up to eight
#   **floating private IP** (x.x.x.12) that initially lives on
#   eni‑blue.
# • Allocates or attaches 1‑8 **floating private IPs** with associated
#   elastic IPs (EIPs). `allow_reassociation=true` lets the
#   same association survive a fail‑over.
# • Two Auto Scaling Groups (1 instance each) ensure one EC2 per
#   ENI at all times. Instances carry an IAM role that allows
#   them to run the fail‑over logic (ReplaceRoute, move private
#   IP, re‑associate EIPs).
# • Adds additional configuration knobs (CloudWatch agent, SSM, SSH
#   key, extra SGs, sizing, etc.).
##############################################################