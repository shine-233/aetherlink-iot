const fs = require('fs');
const path = require('path');

const dir = './coverage/.tmp';
const files = fs.readdirSync(dir).filter(f => f.endsWith('.json'));
let allResults = [];
files.forEach(f => {
  const d = JSON.parse(fs.readFileSync(path.join(dir, f), 'utf8'));
  if (d.result) allResults = allResults.concat(d.result);
});

const targets = [
  'telemetry/telemetry.vue',
  'telemetry/modules/history-data.vue',
  'telemetry/modules/time-series-data.vue',
  'details/modules/give-an-alarm.vue',
  'details/modules/message.vue',
  'details/modules/device-status.vue',
  'shared-with-me/index.vue',
  'warning-message/components/pop-up.vue',
  'warning-message/components/new-information.vue',
  'dashboard/rdi-overview/index.vue'
];

// Group results by url
const byUrl = {};
allResults.filter(r => targets.some(t => r.url.includes(t))).forEach(r => {
  if (!byUrl[r.url]) byUrl[r.url] = [];
  byUrl[r.url].push(r);
});

Object.keys(byUrl).forEach(url => {
  const results = byUrl[url];
  const filePath = url.replace('file:///', '').replace(/\//g, path.sep);
  let src = '';
  try { src = fs.readFileSync(filePath, 'utf8'); } catch (e) { console.log('CANNOT READ: ' + url); return; }

  const lines = src.split('\n');
  const lineStartOffsets = [0];
  let cumOffset = 0;
  for (let i = 0; i < lines.length; i++) {
    cumOffset += lines[i].length + 1;
    lineStartOffsets.push(cumOffset);
  }

  // Collect all ranges from all results for this file
  let allRanges = [];
  results.forEach(r => {
    r.functions.forEach(fn => {
      fn.ranges.forEach(range => {
        allRanges.push({ start: range.startOffset, end: range.endOffset, count: range.count });
      });
    });
  });

  // For each code line, check if it's covered in ANY result
  const codeLines = [];
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i].trim();
    if (line && !line.startsWith('//') && !line.startsWith('*') && !line.startsWith('/*') && !line.startsWith('<!--')) {
      codeLines.push(i + 1);
    }
  }

  const uncoveredLines = [];
  for (const lineNum of codeLines) {
    const lineStart = lineStartOffsets[lineNum - 1];
    const lineEnd = lineStartOffsets[lineNum] - 1;
    const midOffset = Math.floor((lineStart + lineEnd) / 2);

    // Check if covered in any result
    let covered = false;
    for (const r of results) {
      let allR = [];
      r.functions.forEach(fn => {
        fn.ranges.forEach(range => {
          allR.push({ start: range.startOffset, end: range.endOffset, count: range.count });
        });
      });

      // Find innermost range containing midOffset
      let innermost = null;
      for (const ar of allR) {
        if (ar.start <= midOffset && midOffset < ar.end) {
          if (!innermost) {
            innermost = ar;
          } else if (ar.start >= innermost.start && ar.end <= innermost.end && (ar.start > innermost.start || ar.end < innermost.end)) {
            innermost = ar;
          }
        }
      }

      if (innermost && innermost.count > 0) {
        covered = true;
        break;
      }
    }

    if (!covered) {
      uncoveredLines.push(lineNum);
    }
  }

  const totalCodeLines = codeLines.length;
  const coveredCount = totalCodeLines - uncoveredLines.length;
  const pct = Math.round(coveredCount / totalCodeLines * 100);

  const shortPath = url.split('src')[1];
  console.log(shortPath + ' | lines: ' + coveredCount + '/' + totalCodeLines + ' (' + pct + '%) | uncovered: ' + JSON.stringify(uncoveredLines));
});
