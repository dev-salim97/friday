const prefix = "friday.assistant.";

export function readComposerPreference(name) {
  try {
    return localStorage.getItem(prefix + name);
  } catch {
    return null;
  }
}

export function writeComposerPreference(name, value) {
  try {
    localStorage.setItem(prefix + name, value);
  } catch {
    // The selection still applies to the current page.
  }
}
