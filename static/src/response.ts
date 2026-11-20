// Quote unsafe integer tokens before JSON.parse can round uint64 node IDs.
// String tokens are consumed whole so message content and URLs stay unchanged.
export function parseResponse(text: string): unknown {
  const exact = text.replace(
    /"(?:\\.|[^"\\])*"|-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?/g,
    (token) => {
      const isInteger = /^-?\d+$/.test(token);

      if (isInteger && !Number.isSafeInteger(Number(token))) {
        return JSON.stringify(token);
      }

      return token;
    },
  );

  return JSON.parse(exact);
}

export function formatResponse(text: string): string {
  if (!text) {
    return "Operation completed.";
  }

  try {
    return JSON.stringify(parseResponse(text), null, 2);
  } catch {
    return text;
  }
}
