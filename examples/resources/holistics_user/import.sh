# Import by integer user ID:
terraform import holistics_user.alice 123

# Or by email address (more convenient — the Holistics UI doesn't expose user IDs prominently):
terraform import holistics_user.alice alice@example.com
