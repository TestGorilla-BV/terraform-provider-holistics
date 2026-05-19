# Import by integer group ID:
terraform import holistics_group.analysts 42

# Or by group name — looked up via the /groups list. Friendlier when the
# Holistics admin UI shows names rather than IDs:
terraform import holistics_group.analysts Analysts
