# Issues

## Open

### [trx-8csx] Standardize category naming convention (P1, epic)
# Problem
Category 'social+media' uses inconsistent naming with a + sign while others use simple names or hyphens.

# Proposed Solution
- Change 'social+media' to 'social-media' for consistency
...


### [trx-evw1] Remove redundant --link flag alias (P1, epic)
# Problem
Two flags --link and --links-only (-L) do the same thing, causing confusion.

# Proposed Solution
- Remove --link flag
...


### [trx-jjx4] Rename -t flag from time-range to -r (P1, epic)
# Problem
-t flag is used for --time-range but conflicts with common convention of -t for --text

# Proposed Solution
- Change --time-range short flag from -t to -r (range)
...


### [trx-ngye] Change default behavior to non-interactive mode (P1, epic)
# Problem
Interactive TUI mode is currently the default, requiring -p/--np flag for scripting usage.

# Proposed Solution
- Make non-interactive the default behavior
...


### [trx-pmq3] Add default output format to config (P2, epic)
# Problem
Config file lacks option to set default output format.

# Proposed Solution
Add to config.toml:
...


### [trx-7r7h] Add query history feature (P2, epic)
# Problem
No query history feature - users must retype previous searches.

# Proposed Solution
- Add sx history command to show recent searches
...


### [trx-gqz0] Add shell completion support (P2, epic)
# Problem
No shell completions available for bash, zsh, or fish.

# Proposed Solution
- Add sx completion <shell> command using cobra's completion generator
...


## Closed

- [trx-fkdr] Fix version flag to display version instead of searching (closed 2026-02-12)
