const crypto = require('crypto');

function generateCode(length = 8) {
  return crypto.randomBytes(length).toString('base64url').slice(0, length).toUpperCase();
}

module.exports = { generateCode };
