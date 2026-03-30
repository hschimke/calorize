import { api } from './api.js';
import { getLocalDateString } from './utils.js';
import { drawMacroBar, drawMealBars, drawDayBars, drawWeekBars } from './charts.js';
import { renderCalorieGoalBar } from './ui.js';

let lastPeriod = 'day';
let lastDate = null;
let lastStats = null;
let lastExtra = null;
let profile = null;

async function init() {
    const buttons = document.querySelectorAll('.period-btn');
    buttons.forEach(btn => {
        btn.addEventListener('click', () => {
            buttons.forEach(b => b.classList.remove('active'));
            btn.classList.add('active');
            loadStats(btn.dataset.period);
        });
    });

    let resizeTimer;
    window.addEventListener('resize', () => {
        clearTimeout(resizeTimer);
        resizeTimer = setTimeout(redraw, 100);
    });

    profile = await api.getProfile().catch(() => null);
    loadStats('day');
}

async function loadStats(period, date) {
    if (!date) date = getLocalDateString();
    lastPeriod = period;
    lastDate = date;

    updateDisplay({ calories: 0, protein: 0, carbs: 0, fat: 0 });

    try {
        if (period === 'day') {
            const [stats, logs] = await Promise.all([
                api.getStats(period, date),
                api.getLogs(date),
            ]);
            lastStats = stats || { calories: 0, protein: 0, carbs: 0, fat: 0 };
            lastExtra = logs || [];
        } else {
            const [stats, breakdown] = await Promise.all([
                api.getStats(period, date),
                api.getStatsBreakdown(period, date),
            ]);
            lastStats = stats || { calories: 0, protein: 0, carbs: 0, fat: 0 };
            lastExtra = breakdown || [];
        }
    } catch (e) {
        console.error('Failed to load stats', e);
        lastStats = { calories: 0, protein: 0, carbs: 0, fat: 0 };
        lastExtra = [];
    }

    updateDisplay(lastStats);
    showPanels(period);
    redraw();
}

function redraw() {
    if (!lastStats) return;
    const { protein = 0, carbs = 0, fat = 0 } = lastStats;

    drawMacroBar(document.getElementById('chart-macro'), { protein, carbs, fat });

    if (lastPeriod === 'day') {
        const meals = { breakfast: 0, lunch: 0, dinner: 0, snack: 0 };
        (lastExtra || []).forEach(e => {
            const tag = e.meal_tag || 'snack';
            meals[tag] = (meals[tag] || 0) + (e.calories ?? 0);
        });
        drawMealBars(document.getElementById('chart-meals'), meals);
    } else if (lastPeriod === 'week') {
        drawDayBars(document.getElementById('chart-days'), lastExtra || []);
    } else if (lastPeriod === 'month') {
        drawWeekBars(document.getElementById('chart-weeks'), lastExtra || []);
    }
}

function showPanels(period) {
    document.getElementById('panel-macro').style.display = '';
    document.getElementById('panel-meals').style.display = period === 'day' ? '' : 'none';
    document.getElementById('panel-days').style.display = period === 'week' ? '' : 'none';
    document.getElementById('panel-weeks').style.display = period === 'month' ? '' : 'none';
}

function updateDisplay(stats) {
    document.getElementById('stat-calories').textContent = Math.round(stats.calories || 0);
    document.getElementById('stat-protein').textContent = Math.round(stats.protein || 0);
    document.getElementById('stat-carbs').textContent = Math.round(stats.carbs || 0);
    document.getElementById('stat-fat').textContent = Math.round(stats.fat || 0);

    const dailyGoal = profile?.calorie_goal ?? null;
    let scaledGoal = null;
    if (dailyGoal != null) {
        if (lastPeriod === 'week') scaledGoal = dailyGoal * 7;
        else if (lastPeriod === 'month') scaledGoal = Math.round(dailyGoal * 30.4);
        else scaledGoal = dailyGoal;
    }

    const calCard = document.getElementById('stat-calories').parentElement;
    calCard.querySelector('.goal-bar-wrap')?.remove();
    const bar = renderCalorieGoalBar(Math.round(stats.calories || 0), scaledGoal);
    if (bar) calCard.appendChild(bar);
}

window.addEventListener('load', init);
