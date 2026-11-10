// The homepage types one line at a time, so a string can end at the cursor.
const homeToken = /(?<command>^\$.*)|(?<comment>#.*)|(?<string>"(?:\\.|[^"\\])*"?|'(?:\\.|[^'\\])*'?)|(?<keyword>\b(?:class|def|return|self)\b)|(?<property>[A-Za-z_]\w*)|[^#"'A-Za-z_]+/gu;

export function homeCodeTokens(line) {
    return Array.from(line.matchAll(homeToken), match => {
        const kind = Object.keys(match.groups).find(name => match.groups[name] !== undefined);
        return {text: match[0], kind: kind || 'plain'};
    });
}
