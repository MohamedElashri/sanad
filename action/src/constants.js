"use strict";

const packageMetadata = require("../package.json");

module.exports = {
  SANAD_VERSION: packageMetadata.version,
  REPORT_VERSIONS: {
    check: 1,
    plan: 1,
    apply: 1,
    upgrade: 2,
  },
};
