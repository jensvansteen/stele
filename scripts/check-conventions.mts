import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";

const branchPattern = /^(?:main|(?:feat|fix|docs|refactor|perf|test|build|ci|chore)\/[a-z0-9]+(?:-[a-z0-9]+)*|release\/\d+\.\d+\.\d+(?:-[a-z0-9]+(?:\.[a-z0-9]+)*)?)$/;
const commitPattern = /^(?:feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert)(?:\([a-z0-9][a-z0-9-]*\))?!?: \S.+$/;

function check(value: string, pattern: RegExp, label: string): void {
  if (!pattern.test(value)) {
    throw new Error(`${label} does not follow the repository convention: ${value}`);
  }
}

const mode: string | undefined = process.argv[2];
try {
  if (mode === "branch") {
    check(process.argv[3] ?? "", branchPattern, "Branch name");
  } else if (mode === "message-file") {
    const filename: string = process.argv[3] ?? "";
    check(readFileSync(filename, "utf8").split("\n", 1)[0] ?? "", commitPattern, "Commit subject");
  } else if (mode === "ci") {
    check(process.env.PR_BRANCH ?? "", branchPattern, "Branch name");
    check(process.env.PR_TITLE ?? "", commitPattern, "Pull request title");
    const base: string = process.env.BASE_SHA ?? "";
    const head: string = process.env.HEAD_SHA ?? "";
    const subjects: string = execFileSync("git", ["log", "--format=%s", `${base}..${head}`], {
      encoding: "utf8",
    });
    for (const subject of subjects.trim().split("\n")) {
      check(subject, commitPattern, "Commit subject");
    }
  } else {
    throw new Error("Expected branch, message-file, or ci mode");
  }
} catch (error: unknown) {
  console.error(error instanceof Error ? error.message : error);
  process.exitCode = 1;
}
