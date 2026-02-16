(function () {
  "use strict";

  let costChart = null;

  // --- Helpers ---

  function formatUSD(val) {
    if (val == null) return "$0.00";
    return "$" + Number(val).toFixed(4);
  }

  function formatTokens(val) {
    if (val == null) return "0";
    return Number(val).toLocaleString();
  }

  function formatDuration(ms) {
    if (ms == null) return "-";
    if (ms < 1000) return ms + "ms";
    return (ms / 1000).toFixed(2) + "s";
  }

  function formatTime(ts) {
    if (!ts) return "-";
    var d = new Date(ts);
    return d.toLocaleTimeString();
  }

  function budgetColor(pct) {
    if (pct < 60) return "green";
    if (pct < 85) return "yellow";
    return "red";
  }

  async function fetchJSON(url) {
    var res = await fetch(url);
    if (!res.ok) throw new Error("HTTP " + res.status);
    return res.json();
  }

  function showError(container, msg) {
    if (typeof container === "string") {
      container = document.getElementById(container);
    }
    if (!container) return;
    container.innerHTML = '<div class="error-msg">' + msg + "</div>";
  }

  // --- Renderers ---

  function renderSummaryCards(data) {
    var el = document.getElementById("summary-cards");
    el.innerHTML = [
      { label: "Total Requests", value: formatTokens(data.total_requests) },
      { label: "Total Cost", value: formatUSD(data.total_cost_usd) },
      { label: "Unique Agents", value: data.unique_agents || 0 },
      { label: "Avg Latency", value: formatDuration(data.avg_duration_ms) },
    ]
      .map(function (c) {
        return (
          '<div class="summary-card">' +
          '<div class="label">' +
          c.label +
          "</div>" +
          '<div class="value">' +
          c.value +
          "</div>" +
          "</div>"
        );
      })
      .join("");
  }

  function renderCostChart(data) {
    var ctx = document.getElementById("cost-chart").getContext("2d");
    var labels = data.map(function (d) {
      return d.date;
    });
    var costs = data.map(function (d) {
      return d.cost_usd;
    });

    if (costChart) {
      costChart.data.labels = labels;
      costChart.data.datasets[0].data = costs;
      costChart.update();
      return;
    }

    costChart = new Chart(ctx, {
      type: "line",
      data: {
        labels: labels,
        datasets: [
          {
            label: "Daily Cost (USD)",
            data: costs,
            borderColor: "#5dade2",
            backgroundColor: "rgba(93,173,226,0.1)",
            fill: true,
            tension: 0.3,
            pointRadius: 2,
          },
        ],
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        plugins: {
          legend: { display: false },
        },
        scales: {
          x: {
            ticks: { color: "#8888aa", maxTicksLimit: 10 },
            grid: { color: "#2a2a4a" },
          },
          y: {
            ticks: {
              color: "#8888aa",
              callback: function (v) {
                return "$" + v.toFixed(2);
              },
            },
            grid: { color: "#2a2a4a" },
          },
        },
      },
    });
  }

  function renderAgentsTable(agents) {
    var tbody = document.querySelector("#agents-data tbody");
    if (!agents || agents.length === 0) {
      tbody.innerHTML =
        '<tr><td colspan="5" style="text-align:center;color:#8888aa">No agent data</td></tr>';
      return;
    }
    tbody.innerHTML = agents
      .map(function (a) {
        return (
          "<tr>" +
          "<td>" +
          (a.agent_name || "-") +
          "</td>" +
          "<td>" +
          formatTokens(a.requests) +
          "</td>" +
          "<td>" +
          formatTokens(a.input_tokens) +
          "</td>" +
          "<td>" +
          formatTokens(a.output_tokens) +
          "</td>" +
          "<td>" +
          formatUSD(a.cost_usd) +
          "</td>" +
          "</tr>"
        );
      })
      .join("");
  }

  function renderBudgets(budgets) {
    var el = document.getElementById("budgets-list");
    if (!budgets || Object.keys(budgets).length === 0) {
      el.innerHTML =
        '<div class="error-msg" style="color:#8888aa">No budgets configured</div>';
      return;
    }

    el.innerHTML = Object.keys(budgets)
      .map(function (agent) {
        var b = budgets[agent];
        var dailyPct =
          b.daily_limit_usd > 0
            ? Math.min(100, (b.daily_spend / b.daily_limit_usd) * 100)
            : 0;
        var monthlyPct =
          b.monthly_limit_usd > 0
            ? Math.min(100, (b.monthly_spend / b.monthly_limit_usd) * 100)
            : 0;

        return (
          '<div class="budget-item">' +
          '<div class="budget-header">' +
          '<span class="budget-agent">' +
          agent +
          "</span>" +
          "</div>" +
          '<div class="budget-row">' +
          "<div>" +
          '<div class="budget-label">Daily ' +
          formatUSD(b.daily_spend) +
          " / " +
          formatUSD(b.daily_limit_usd) +
          "</div>" +
          '<div class="budget-bar-track">' +
          '<div class="budget-bar-fill ' +
          budgetColor(dailyPct) +
          '" style="width:' +
          dailyPct +
          '%"></div>' +
          "</div>" +
          "</div>" +
          "<div>" +
          '<div class="budget-label">Monthly ' +
          formatUSD(b.monthly_spend) +
          " / " +
          formatUSD(b.monthly_limit_usd) +
          "</div>" +
          '<div class="budget-bar-track">' +
          '<div class="budget-bar-fill ' +
          budgetColor(monthlyPct) +
          '" style="width:' +
          monthlyPct +
          '%"></div>' +
          "</div>" +
          "</div>" +
          "</div>" +
          "</div>"
        );
      })
      .join("");
  }

  function renderRecentRequests(logs) {
    var tbody = document.querySelector("#requests-data tbody");
    if (!logs || logs.length === 0) {
      tbody.innerHTML =
        '<tr><td colspan="7" style="text-align:center;color:#8888aa">No recent requests</td></tr>';
      return;
    }
    tbody.innerHTML = logs
      .map(function (r) {
        var totalTokens = (r.input_tokens || 0) + (r.output_tokens || 0);
        var statusClass =
          r.status_code >= 200 && r.status_code < 400
            ? "status-ok"
            : "status-err";
        return (
          "<tr>" +
          "<td>" +
          formatTime(r.timestamp) +
          "</td>" +
          "<td>" +
          (r.agent_name || "-") +
          "</td>" +
          "<td>" +
          (r.model || "-") +
          "</td>" +
          "<td>" +
          formatTokens(totalTokens) +
          "</td>" +
          "<td>" +
          formatUSD(r.cost_usd) +
          "</td>" +
          '<td class="' +
          statusClass +
          '">' +
          (r.status_code || "-") +
          "</td>" +
          "<td>" +
          formatDuration(r.duration_ms) +
          "</td>" +
          "</tr>"
        );
      })
      .join("");
  }

  // --- Data loading ---

  async function loadAll() {
    var results = await Promise.allSettled([
      fetchJSON("/api/stats"),
      fetchJSON("/api/agents"),
      fetchJSON("/api/budgets"),
      fetchJSON("/api/costs/daily"),
      fetchJSON("/api/logs"),
    ]);

    if (results[0].status === "fulfilled") {
      renderSummaryCards(results[0].value);
    } else {
      showError("summary-cards", "Error loading data");
    }

    if (results[1].status === "fulfilled") {
      renderAgentsTable(results[1].value);
    } else {
      showError(
        document.querySelector("#agents-data tbody"),
        "Error loading data"
      );
    }

    if (results[2].status === "fulfilled") {
      renderBudgets(results[2].value);
    } else {
      showError("budgets-list", "Error loading data");
    }

    if (results[3].status === "fulfilled") {
      renderCostChart(results[3].value);
    } else {
      showError("cost-chart-container", "Error loading data");
    }

    if (results[4].status === "fulfilled") {
      renderRecentRequests(results[4].value);
    } else {
      showError(
        document.querySelector("#requests-data tbody"),
        "Error loading data"
      );
    }
  }

  // --- Init ---

  loadAll();
  setInterval(loadAll, 5000);
})();
