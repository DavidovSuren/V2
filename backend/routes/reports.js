const express = require('express');
const { identify, requireUser } = require('../lib/telegramAuth');
const { wrap } = require('../lib/asyncHandler');
const { weeklyReportFor, monthlyReportFor } = require('../lib/reports');

const router = express.Router();

router.get('/reports/weekly', identify, requireUser, wrap(async (req, res) => {
  res.json(await weeklyReportFor(req.user.id));
}));

router.get('/reports/monthly', identify, requireUser, wrap(async (req, res) => {
  res.json(await monthlyReportFor(req.user.id));
}));

module.exports = router;
