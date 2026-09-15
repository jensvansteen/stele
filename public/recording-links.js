export function recordingsForRequirement(recordings, requirement) {
  const scenarioIds = new Set((requirement.scenarios ?? []).map((scenario) => scenario.id));
  return recordings
    .map((recording) => ({
      ...recording,
      matchingScenarioIds: (recording.coveredScenarios ?? []).filter((id) => scenarioIds.has(id))
    }))
    .filter((recording) => recording.matchingScenarioIds.length > 0);
}
