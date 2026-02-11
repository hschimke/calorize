import { api } from './api.js';

async function loadFoods() {
    try {
        const foods = await api.getFoods();
        const foodsList = document.getElementById('foods-list');
        foodsList.innerHTML = '';

        if (foods && foods.length > 0) {
            foods.forEach(food => {
                const li = document.createElement('li');
                li.innerHTML = `
                    <div class="food-info">
                        <strong>${food.name}</strong><br>
                        <small>${Math.round(food.calories)} kcal | P: ${Math.round(food.protein)}g | C: ${Math.round(food.carbs)}g | F: ${Math.round(food.fat)}g</small>
                        ${food.nutrients && food.nutrients.length > 0 ?
                        `<br><small><em>${food.nutrients.map(n => `${n.name}: ${n.amount}${n.unit}`).join(', ')}</em></small>`
                        : ''}
                    </div>
                `;
                // Add delete button
                const deleteBtn = document.createElement('button');
                deleteBtn.textContent = 'Delete';
                deleteBtn.onclick = () => deleteFood(food.id);
                li.appendChild(deleteBtn);

                foodsList.appendChild(li);
            });
        } else {
            foodsList.innerHTML = '<li>No foods found.</li>';
        }
    } catch (e) {
        console.error("Failed to load foods:", e);
    }
}

function addNutrientRow() {
    const container = document.getElementById('nutrients-container');
    const row = document.createElement('div');
    row.className = 'nutrient-row';
    row.innerHTML = `
        <input type="text" placeholder="Name (e.g. Vitamin C)" class="nutrient-name" required>
        <input type="number" placeholder="Amount" class="nutrient-amount" step="0.1" required>
        <input type="text" placeholder="Unit (e.g. mg)" class="nutrient-unit" required>
        <button type="button" class="remove-btn" onclick="this.parentElement.remove()">×</button>
    `;
    container.appendChild(row);
}




async function deleteFood(id) {
    if (!confirm("Are you sure you want to delete this food?")) return;
    try {
        await api.deleteFood(id);
        loadFoods();
    } catch (e) {
        alert("Failed to delete food: " + e.message);
    }
}

// --- Recipe Logic ---

let availableFoods = [];
let recipeIngredients = []; // Array of {id, name, amount, unit}

window.toggleFoodType = function () {
    console.log("Toggling food type");
    const type = document.querySelector('input[name="type"]:checked').value;
    const macrosSection = document.getElementById('macros-section');
    const ingredientsSection = document.getElementById('ingredients-section');
    const inputs = macrosSection.querySelectorAll('input');

    if (type === 'recipe') {
        macrosSection.style.display = 'none';
        ingredientsSection.style.display = 'block';
        inputs.forEach(i => i.removeAttribute('required'));
    } else {
        macrosSection.style.display = 'block';
        ingredientsSection.style.display = 'none';
        inputs.forEach(i => i.setAttribute('required', ''));
    }
}

function updateIngredientList() {
    const list = document.getElementById('ingredients-list');
    list.innerHTML = '';
    recipeIngredients.forEach((ing, index) => {
        const div = document.createElement('div');
        div.className = 'nutrient-row'; // Reuse style
        div.innerHTML = `
            <span>${ing.name}</span>
            <span>${ing.amount} ${ing.unit}</span>
            <button type="button" class="remove-btn" onclick="removeIngredient(${index})">×</button>
        `;
        list.appendChild(div);
    });
}

window.removeIngredient = function (index) {
    recipeIngredients.splice(index, 1);
    updateIngredientList();
}

window.addIngredient = function () {
    const select = document.getElementById('ingredient-search');
    const amountInput = document.getElementById('ingredient-amount');
    const foodId = select.value;
    const amount = parseFloat(amountInput.value);

    if (!foodId || isNaN(amount) || amount <= 0) {
        alert("Please select a food and enter a valid amount.");
        return;
    }

    const food = availableFoods.find(f => f.id === foodId);
    if (!food) return;

    recipeIngredients.push({
        id: food.id,
        name: food.name,
        amount: amount,
        unit: food.measurement_unit // Simplified: assuming added amount matches base unit or is just a scalar
    });

    updateIngredientList();
    select.value = "";
    amountInput.value = "";
}

function populateIngredientSearch(foods) {
    availableFoods = foods; // Store for lookup
    const select = document.getElementById('ingredient-search');
    select.innerHTML = '<option value="">Select a food...</option>';
    // Only allow non-recipes as ingredients for now to avoid cycles/complexity, or allow all?
    // Let's allow all for maximum flexibility, backend should handle cycles (or stack overflow :P)
    foods.forEach(f => {
        const option = document.createElement('option');
        option.value = f.id;
        option.textContent = f.name;
        select.appendChild(option);
    });
}

async function createFood(e) {
    e.preventDefault();
    const form = e.target;
    const type = document.querySelector('input[name="type"]:checked').value;

    let foodData = {
        name: form.name.value,
        type: type,
        measurement_unit: 'serving', // Default for now
        measurement_amount: 1,
    };

    if (type === 'recipe') {
        if (recipeIngredients.length === 0) {
            alert("Please add at least one ingredient for the recipe.");
            return;
        }
        // Calculate estimated macros (backend doesn't do this automatically yet on create)
        // Actually, backend MIGHT not reject if calories are 0, but let's try to sum them up for display correctness
        let cal = 0, p = 0, c = 0, f = 0;
        const ingredientsMap = {};

        recipeIngredients.forEach(ing => {
            ingredientsMap[ing.id] = ing.amount;
            const food = availableFoods.find(f => f.id === ing.id);
            if (food) {
                // assume food nutrients are per measurement_amount
                // factor = amount / food.measurement_amount
                // wait, simplistic assumption: food macros are for 'measurement_amount'
                // and ingredient amount is in standard units? 
                // Let's assume input amount is consistent with food's unit.
                const factor = ing.amount / (food.measurement_amount || 100);
                cal += food.calories * factor;
                p += food.protein * factor;
                c += food.carbs * factor;
                f += food.fat * factor;
            }
        });

        foodData.calories = cal;
        foodData.protein = p;
        foodData.carbs = c;
        foodData.fat = f;
        foodData.ingredients = ingredientsMap;

    } else {
        foodData.calories = parseFloat(form.calories.value);
        foodData.protein = parseFloat(form.protein.value);
        foodData.carbs = parseFloat(form.carbs.value);
        foodData.fat = parseFloat(form.fat.value);

        // Collect nutrients
        const nutrients = [];
        document.querySelectorAll('.nutrient-row').forEach(row => {
            // Check if it's a nutrient row input (not a simple text span from recipes)
            const nameInput = row.querySelector('.nutrient-name');
            if (nameInput) {
                const name = nameInput.value;
                const amount = parseFloat(row.querySelector('.nutrient-amount').value);
                const unit = row.querySelector('.nutrient-unit').value;
                if (name && !isNaN(amount) && unit) {
                    nutrients.push({ name, amount, unit });
                }
            }
        });
        foodData.nutrients = nutrients;
    }

    try {
        await api.createFood(foodData);
        form.reset();
        document.getElementById('nutrients-container').innerHTML = ''; // Clear nutrients
        recipeIngredients = []; // Clear ingredients
        updateIngredientList();
        // Reset to food type
        document.getElementById('type-food').checked = true;
        toggleFoodType();

        loadFoods();
    } catch (e) {
        alert("Failed to create food: " + e.message);
    }
}

window.addEventListener('load', () => {
    // Modify loadFoods to also populate search
    const originalLoadFoods = loadFoods;
    loadFoods = async function () {
        try {
            const foods = await api.getFoods();
            populateIngredientSearch(foods || []);
            // Render list
            const foodsList = document.getElementById('foods-list');
            foodsList.innerHTML = '';
            if (foods && foods.length > 0) {
                foods.forEach(food => {
                    const li = document.createElement('li');
                    let details = `${Math.round(food.calories)} kcal | P: ${Math.round(food.protein)}g | C: ${Math.round(food.carbs)}g | F: ${Math.round(food.fat)}g`;
                    if (food.type === 'recipe') {
                        details += ` <span style="background:#eee; padding:2px 6px; border-radius:4px; font-size:0.8em;">Recipe</span>`;
                    }

                    li.innerHTML = `
                        <div class="food-info">
                            <strong>${food.name}</strong><br>
                            <small>${details}</small>
                            ${food.nutrients && food.nutrients.length > 0 ?
                            `<br><small><em>${food.nutrients.map(n => `${n.name}: ${n.amount}${n.unit}`).join(', ')}</em></small>`
                            : ''}
                        </div>
                    `;
                    const deleteBtn = document.createElement('button');
                    deleteBtn.textContent = 'Delete';
                    deleteBtn.onclick = () => deleteFood(food.id);
                    li.appendChild(deleteBtn);
                    foodsList.appendChild(li);
                });
            } else {
                foodsList.innerHTML = '<li>No foods found.</li>';
            }
        } catch (e) { console.error(e); }
    };

    loadFoods();
    // Initialize toggle state
    toggleFoodType();
    document.getElementById('create-food-form').addEventListener('submit', createFood);
    document.getElementById('add-nutrient-btn').addEventListener('click', addNutrientRow);
});
