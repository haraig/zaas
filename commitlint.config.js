export default {
  extends: ["@commitlint/config-conventional"],
  // Dependabot's auto-generated commit body (release notes links, etc.) routinely
  // exceeds body-max-line-length and can't be shortened via dependabot.yaml.
  ignores: [(message) => message.includes("Signed-off-by: dependabot[bot]")],
};
