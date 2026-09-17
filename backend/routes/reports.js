const express = require('express');
const { identify, requireUser } = require('../lib/telegramAuth');
const { weeklyReportFor, monthlyReportFor } = require('../lib/reports');

const router = express.Router();

router.get('/reports/weekly', identify, requireUser, (req, res) => {
  res.json(weeklyReportFor(req.user.id));
});

router.get('/reports/monthly', identify, requireUser, (req, res) => {
  res.json(monthlyReportFor(req.user.id));
});

module.exports = router;
