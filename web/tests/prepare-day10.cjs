const fs = require('node:fs')
const fixture = JSON.parse(fs.readFileSync('.local/day10-browser.json', 'utf8'))
const scenario = process.argv[2] === 'day11' ? 'day11' : 'day10'
const source = fs.readFileSync('web/tests/' + scenario + '-browser.js', 'utf8')
fs.writeFileSync('.local/' + scenario + '-browser-run.js', source.replace('/* DAY10_FIXTURE */ null', JSON.stringify(fixture)), { mode: 0o600 })
console.log('Prepared local browser fixture script')
