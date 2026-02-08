import { api } from './api.js';

async function init() {
    // Period buttons
    const buttons = document.querySelectorAll('.period-btn');
    buttons.forEach(btn => {
        btn.addEventListener('click', () => {
            // update active state
            buttons.forEach(b => b.classList.remove('active'));
            btn.classList.add('active');

            // load stats
            loadStats(btn.dataset.period);
        });
    });

    // Initial load
    loadStats('day');
}

async function loadStats(period) {
    try {
        const stats = await api.getStats(period);

        // Reset to 0
        updateDisplay({ calories: 0, protein: 0, carbs: 0, fat: 0 });

        if (stats) {
            updateDisplay(stats);
        }
    } catch (e) {
        console.error("Failed to load stats", e);
    }
}

function updateDisplay(stats) {
    document.getElementById('stat-calories').textContent = Math.round(stats.calories || 0);
    document.getElementById('stat-protein').textContent = Math.round(stats.protein || 0);
    document.getElementById('stat-carbs').textContent = Math.round(stats.carbs || 0);
    document.getElementById('stat-fat').textContent = Math.round(stats.fat || 0);
}

window.addEventListener('load', init);
