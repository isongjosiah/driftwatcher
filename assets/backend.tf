terraform {
  backend "local" {
    path = "terraform.tfstate" # Path to the local state file
  }
}
