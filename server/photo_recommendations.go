package main

import (
	"fmt"
)

// RecommendationResponse represents the final response to client for photo classify API.
type RecommendationResponse struct {
	Success bool `json:"success"`
	ClassificationResult
	Recommendation string `json:"recommendation,omitempty"`
	Error          string `json:"error,omitempty"`
}

// Формирует текстовую рекомендацию по результату CV через LLM (или шаблон).
func generateRecommendation(classification *ClassificationResult, cropID string) (string, error) {
	prompts := promptsForCrop(cropID)
	prompt := fmt.Sprintf(`%s
Based on the following classification result from an image analysis, provide detailed care recommendations.

Classification Result:
- Detected: %s
- Confidence: %.2f%%
- Top predictions: %v

Please provide:
1. A brief explanation of what was detected
2. Specific care recommendations for this condition
3. Preventive measures if it's a disease
4. Treatment options if applicable
5. General tips for maintaining healthy trees

Respond in Russian language as the target audience is Russian-speaking gardeners.`,
		prompts.PhotoUserIntro,
		classification.Prediction,
		classification.Confidence*100,
		classification.TopPredictions,
	)

	if config.LLMAPIKey == "" {
		return generateTemplateRecommendation(classification), nil
	}
	return callLLMCompletion([]Message{
		{Role: "system", Content: prompts.PhotoSystem},
		{Role: "user", Content: prompt},
	})
}

// generateRecommendationWithHistory — рекомендации по фото с учётом предыдущих реплик диалога.
func generateRecommendationWithHistory(classification *ClassificationResult, caption string, history []Message, cropID string) (string, error) {
	prompts := promptsForCrop(cropID)
	prompt := fmt.Sprintf(`%s
Based on the following classification result from an image analysis, provide detailed care recommendations.

Classification Result:
- Detected: %s
- Confidence: %.2f%%
- Top predictions: %v

User caption (may be empty): %s

Please provide:
1. A brief explanation of what was detected
2. Specific care recommendations for this condition
3. Preventive measures if it's a disease
4. Treatment options if applicable
5. General tips for maintaining healthy trees

Respond in Russian language as the target audience is Russian-speaking gardeners.`,
		prompts.PhotoUserIntro,
		classification.Prediction,
		classification.Confidence*100,
		classification.TopPredictions,
		caption,
	)
	if config.LLMAPIKey == "" {
		return generateTemplateRecommendation(classification), nil
	}
	msgs := make([]Message, 0, len(history)+2)
	msgs = append(msgs, Message{Role: "system", Content: prompts.PhotoSystem})
	msgs = append(msgs, history...)
	msgs = append(msgs, Message{Role: "user", Content: prompt})
	return callLLMCompletion(msgs)
}

// Статичные рекомендации по метке класса, если LLM недоступен.
func generateTemplateRecommendation(classification *ClassificationResult) string {
	recommendations := map[string]string{
		"healthy_apple": `🍎 Здоровое яблоко обнаружено!

Что выявлено:
Ваше яблоко выглядит здоровым, без видимых признаков заболеваний.

Рекомендации по уходу:
• Продолжайте регулярный полив (2-3 раза в неделю)
• Вносите органические удобрения каждые 4-6 недель
• Проводите профилактическую обрезку сухих ветвей
• Следите за появлением вредителей
• Собирайте урожай вовремя для лучшего качества`,

		"apple_scab": `🍂 Обнаружена парша яблони!

Что выявлено:
Парша - грибковое заболевание, поражающее листья и плоды яблони.

Рекомендации по уходу:
• Удалите и сожгите все поражённые листья и плоды
• Обработайте дерево фунгицидами (бордоская жидкость, медный купорос)
• Проведите обработку ранней весной до распускания почек
• Осенью уберите всю опавшую листву
• Прореживайте крону для лучшей вентиляции

Профилактика:
• Выбирайте устойчивые сорта
• Регулярно проводите профилактические обработки
• Поддерживайте чистоту приствольного круга`,

		"black_rot": `🖤 Обнаружена чёрная гниль!

Что выявлено:
Чёрная гниль - серьёзное грибковое заболевание плодов.

Рекомендации по уходу:
• Немедленно удалите все поражённые плоды
• Обработайте дерево фунгицидами
• Улучшите циркуляцию воздуха вокруг дерева
• Избегайте повреждения плодов при уходе
• Собирайте урожай аккуратно

Профилактика:
• Регулярная обработка фунгицидами в сезон
• Контроль влажности
• Своевременная уборка урожая`,

		"cedar_apple_rust": `🧡 Обнаружена кедрово-яблоневая ржавчина!

Что выявлено:
Грибковое заболевание, требующее наличия двух хозяев (яблоня и можжевельник).

Рекомендации по уходу:
• По возможности удалите nearby можжевельники
• Обработайте фунгицидами содержащими серу
• Удаляйте поражённые листья
• Проведите обработку весной до цветения

Профилактика:
• Сажайте яблони подальше от можжевельников
• Регулярный осмотр деревьев`,

		"powdery_mildew": `⚪ Обнаружена мучнистая роса!

Что выявлено:
Грибковое заболевание, проявляющееся белым налётом.

Рекомендации по уходу:
• Обработайте раствором соды (1 ст. ложка на 1 л воды)
• Используйте серные препараты
• Удалите сильно поражённые побеги
• Улучшите вентиляцию кроны

Профилактика:
• Не перекармливайте азотными удобрениями
• Соблюдайте режим полива
• Проводите профилактические обработки`,

		"fire_blight": `🔥 Обнаружен бактериальный ожог!

Что выявлено:
Серьёзное бактериальное заболевание, требующее немедленного вмешательства.

Рекомендации по уходу:
• Срочно удалите все поражённые ветви (на 20-30 см ниже поражения)
• Дезинфицируйте инструменты после каждой обрезки
• Обработайте антибиотиками для растений
• При сильном поражении может потребоваться удаление дерева

Профилактика:
• Контроль насекомых-опылителей
• Избегайте обрезки во влажную погоду
• Выбирайте устойчивые сорта`,

		"healthy_leaf": `🌿 Здоровый лист яблони!

Что выявлено:
Листья выглядят здоровыми, признаков заболеваний нет.

Рекомендации по уходу:
• Продолжайте текущий режим ухода
• Регулярно осматривайте дерево
• Поддерживайте оптимальный полив
• Вносите сбалансированные удобрения
• Проводите своевременную обрезку`,

		"default": `🍎 Результаты анализа яблони

Что выявлено:
На основе анализа изображения была определена категория: {{PREDICTION}}
Уверенность классификации: {{CONFIDENCE}}%

Общие рекомендации по уходу за яблоней:
• Регулярный полив (особенно в засушливый период)
• Сезонная обрезка для формирования кроны
• Внесение органических и минеральных удобрений
• Профилактическая обработка от вредителей и болезней
• Мульчирование приствольного круга
• Защита от солнечных ожогов зимой

Для более точных рекомендаций обратитесь к специалисту или загрузите новое изображение.`,
	}

	rec, exists := recommendations[classification.Prediction]
	if !exists {
		rec = recommendations["default"]
		rec = replacePlaceholder(rec, "{{PREDICTION}}", classification.Prediction)
		confStr := fmt.Sprintf("%.1f", classification.Confidence*100)
		rec = replacePlaceholder(rec, "{{CONFIDENCE}}", confStr)
	}
	return rec
}

// Подставляет value вместо placeholder в шаблоне рекомендации.
func replacePlaceholder(str, placeholder, value string) string {
	result := ""
	for i := 0; i < len(str); i++ {
		if i+len(placeholder) <= len(str) && str[i:i+len(placeholder)] == placeholder {
			result += value
			i += len(placeholder) - 1
		} else {
			result += string(str[i])
		}
	}
	return result
}
