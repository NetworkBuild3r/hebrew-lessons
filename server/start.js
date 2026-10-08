const paths = require('./paths');

const seeded = paths.seedLessons();
if (seeded.seeded) {
    console.log('Seeded lessons into ' + seeded.dir);
} else {
    console.log('Lessons directory: ' + seeded.dir);
}

require('./web').listen();
require('../mcp/http').listen();
