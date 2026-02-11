import { api } from './api.js';

let availableFoods = [];
let recipeIngredients = []; // Array of {id, name, amount, unit}
let editingFoodId = null; // null = create mode, string = edit mode

async function loadFoods() {
    try {
        const foods = await api.getFoods();
        populateIngredientSearch(foods || []);
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
                    <div class="food-actions">
                    </div>
                `;
                const actionsDiv = li.querySelector('.food-actions');

                const editBtn = document.createElement('button');
                editBtn.textContent = 'Edit';
                editBtn.className = 'secondary-btn';
                editBtn.style.marginRight = '8px';
                editBtn.onclick = () => startEdit(food.id);
                actionsDiv.appendChild(editBtn);

                const deleteBtn = document.createElement('button');
                deleteBtn.textContent = 'Delete';
                deleteBtn.onclick = () => deleteFood(food.id);
                actionsDiv.appendChild(deleteBtn);

                foodsList.appendChild(li);
            });
        } else {
            foodsList.innerHTML = '<li>No foods found.</li>';
        }
    } catch (e) {
        console.error("Failed to load foods:", e);
    }
}

function addNutrientRow(name, amount, unit) {
    const container = document.getElementById('nutrients-container');
    const row = document.createElement('div');
    row.className = 'nutrient-row';
    row.innerHTML = `
        <input type="text" placeholder="Name (e.g. Vitamin C)" class="nutrient-name" required value="${name || ''}">
        <input type="number" placeholder="Amount" class="nutrient-amount" step="0.1" required value="${amount || ''}">
        <input type="text" placeholder="Unit (e.g. mg)" class="nutrient-unit" required value="${unit || ''}">
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

// --- Edit Logic ---

async function startEdit(foodId) {
    // Ensure available foods are loaded first
    if (availableFoods.length === 0) {
        const foods = await api.getFoods();
        populateIngredientSearch(foods || []);
    }

    // Fetch full food details (includes nutrients and ingredients)
    const food = await api.getFood(foodId);
    if (!food) { alert("Could not load food details."); return; }

    editingFoodId = food.id;
    const form = document.getElementById('create-food-form');

    // Update heading and button
    document.getElementById('form-heading').textContent = 'Edit Food';
    document.getElementById('form-submit-btn').textContent = 'Save Changes';
    document.getElementById('cancel-edit-btn').style.display = 'inline-block';

    // Set name
    form.name.value = food.name;

    // Set type toggle
    if (food.type === 'recipe') {
        document.getElementById('type-recipe').checked = true;
    } else {
        document.getElementById('type-food').checked = true;
    }
    window.toggleFoodType();

    if (food.type === 'recipe') {
        // Populate ingredients
        recipeIngredients = [];
        if (food.ingredients && food.ingredients.length > 0) {
            for (const ing of food.ingredients) {
                let ingredientFood = availableFoods.find(f => f.id === ing.ingredient_id);
                // Fallback: fetch the ingredient food individually if not found in list
                if (!ingredientFood) {
                    try {
                        ingredientFood = await api.getFood(ing.ingredient_id);
                    } catch (err) { /* ignore */ }
                }
                recipeIngredients.push({
                    id: ing.ingredient_id,
                    name: ingredientFood ? ingredientFood.name : '(unknown food)',
                    amount: ing.amount,
                    unit: ingredientFood ? (ingredientFood.measurement_unit || '') : ''
                });
            }
        }
        updateIngredientList();
    } else {
        // Populate macros
        form.calories.value = food.calories;
        form.protein.value = food.protein;
        form.carbs.value = food.carbs;
        form.fat.value = food.fat;

        // Populate nutrients
        document.getElementById('nutrients-container').innerHTML = '';
        if (food.nutrients && food.nutrients.length > 0) {
            food.nutrients.forEach(n => addNutrientRow(n.name, n.amount, n.unit));
        }
    }

    // Scroll to form
    form.scrollIntoView({ behavior: 'smooth' });
}

window.cancelEdit = function () {
    editingFoodId = null;
    const form = document.getElementById('create-food-form');
    form.reset();
    document.getElementById('form-heading').textContent = 'Add New Food';
    document.getElementById('form-submit-btn').textContent = 'Add Food';
    document.getElementById('cancel-edit-btn').style.display = 'none';
    document.getElementById('nutrients-container').innerHTML = '';
    recipeIngredients = [];
    updateIngredientList();
    document.getElementById('type-food').checked = true;
    window.toggleFoodType();
}

// --- Toggle Logic ---

window.toggleFoodType = function () {
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
        div.className = 'nutrient-row';
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
        unit: food.measurement_unit
    });

    updateIngredientList();
    select.value = "";
    amountInput.value = "";
}

function populateIngredientSearch(foods) {
    availableFoods = foods;
    const select = document.getElementById('ingredient-search');
    select.innerHTML = '<option value="">Select a food...</option>';
    foods.forEach(f => {
        const option = document.createElement('option');
        option.value = f.id;
        option.textContent = f.name;
        select.appendChild(option);
    });
}

// --- Submit (Create or Update) ---

async function handleSubmit(e) {
    e.preventDefault();
    const form = e.target;
    const type = document.querySelector('input[name="type"]:checked').value;

    let foodData = {
        name: form.name.value,
        type: type,
        measurement_unit: 'serving',
        measurement_amount: 1,
    };

    if (type === 'recipe') {
        if (recipeIngredients.length === 0) {
            alert("Please add at least one ingredient for the recipe.");
            return;
        }
        let cal = 0, p = 0, c = 0, fat = 0;
        const ingredientsMap = {};

        recipeIngredients.forEach(ing => {
            ingredientsMap[ing.id] = ing.amount;
            const food = availableFoods.find(f => f.id === ing.id);
            if (food) {
                const factor = ing.amount / (food.measurement_amount || 100);
                cal += food.calories * factor;
                p += food.protein * factor;
                c += food.carbs * factor;
                fat += food.fat * factor;
            }
        });

        foodData.calories = cal;
        foodData.protein = p;
        foodData.carbs = c;
        foodData.fat = fat;
        foodData.ingredients = ingredientsMap;
    } else {
        foodData.calories = parseFloat(form.calories.value);
        foodData.protein = parseFloat(form.protein.value);
        foodData.carbs = parseFloat(form.carbs.value);
        foodData.fat = parseFloat(form.fat.value);

        const nutrients = [];
        document.querySelectorAll('#nutrients-container .nutrient-row').forEach(row => {
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
        if (editingFoodId) {
            await api.updateFood(editingFoodId, foodData);
        } else {
            await api.createFood(foodData);
        }

        // Reset form
        editingFoodId = null;
        form.reset();
        document.getElementById('form-heading').textContent = 'Add New Food';
        document.getElementById('form-submit-btn').textContent = 'Add Food';
        document.getElementById('cancel-edit-btn').style.display = 'none';
        document.getElementById('nutrients-container').innerHTML = '';
        recipeIngredients = [];
        updateIngredientList();
        document.getElementById('type-food').checked = true;
        window.toggleFoodType();

        loadFoods();
    } catch (e) {
        alert("Failed to save food: " + e.message);
    }
}

window.addEventListener('load', () => {
    loadFoods();
    window.toggleFoodType();
    document.getElementById('create-food-form').addEventListener('submit', handleSubmit);
    document.getElementById('add-nutrient-btn').addEventListener('click', () => addNutrientRow());
});
