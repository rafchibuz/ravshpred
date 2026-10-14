import readXlsxFile from 'read-excel-file/browser';

const value = (row, index) => String(row[index] ?? '').trim();
const field = (label, content) => ({ label, value: content || 'Не указано' });

function video(title, content) {
  const answer = String(content || '').trim();
  const match = answer.match(/https?:\/\/\S+/i);
  return { title, url: match?.[0] || '', text: match ? '' : answer };
}

export function parseCastingRows(rows) {
  if (!Array.isArray(rows) || rows.length < 2) throw new Error('В Excel нет строк с участниками');
  const headers = rows[0].map((item) => String(item || '').trim().toLowerCase());
  if (!headers[1]?.includes('имя') || !headers[3]?.includes('город') || headers.length < 30) {
    throw new Error('Колонки Excel не соответствуют форме кастинга');
  }
  return rows.slice(1).filter((row) => row.slice(0, 30).some((item) => String(item ?? '').trim())).map((row, index) => {
    const name = value(row, 1);
    if (!name) throw new Error(`В строке ${index + 2} не указано имя`);
    const city = value(row, 3);
    const nickname = value(row, 4);
    const twitch = value(row, 5);
    const age = value(row, 6);
    const height = value(row, 7);
    const weight = value(row, 8);
    return {
      id: index + 1,
      full_name: name,
      nickname,
      twitch,
      age,
      city,
      height: height ? `${height}${/см/i.test(height) ? '' : ' см'}` : '',
      weight: weight ? `${weight}${/кг/i.test(weight) ? '' : ' кг'}` : '',
      sections: [
        {
          title: 'Основная информация',
          fields: [field('Имя', name), field('Город', city), field('Псевдоним / никнейм', nickname), field('Ник на Twitch', twitch), field('Возраст', age), field('Рост', height ? `${height}${/см/i.test(height) ? '' : ' см'}` : ''), field('Вес', weight ? `${weight}${/кг/i.test(weight) ? '' : ' кг'}` : '')],
        },
        {
          title: 'Спортивный профиль',
          fields: [field('Занятие', value(row, 9)), field('Опыт единоборств', value(row, 10)), field('Виды единоборств', value(row, 11)), field('Спортивный разряд', value(row, 12)), field('Официальные соревнования', value(row, 13)), field('Опыт реальных поединков', value(row, 14)), field('Тренируется сейчас', value(row, 15)), field('Официальных боёв', value(row, 16)), field('Тренер / контракт', value(row, 17))],
        },
        {
          title: 'Мотивация и личность',
          fields: [field('Почему хочет попасть в реалити', value(row, 19)), field('Почему выбрать именно его', value(row, 20)), field('Что готов изменить ради участия', value(row, 21))],
        },
        {
          title: 'Готовность к участию',
          fields: [field('Медицинская справка', value(row, 18)), field('Проезд', value(row, 25)), field('Обязательства, которые могут помешать участию', value(row, 26))],
        },
        {
          title: 'Медийность',
          fields: [field('Публичные аккаунты', value(row, 27)), field('Ссылки', value(row, 28)), field('Подписчики суммарно', value(row, 29))],
        },
      ],
      videos: [video('Бой с тенью', value(row, 22)), video('Работа по лапам / груше', value(row, 23)), video('Почему ты?', value(row, 24))],
    };
  });
}

export async function importCastingWorkbook(file) {
  if (!file || !/\.xlsx$/i.test(file.name || '')) throw new Error('Выберите файл Excel в формате .xlsx');
  const workbook = await readXlsxFile(file);
  const rows = Array.isArray(workbook[0]) ? workbook : workbook[0]?.data;
  if (!rows) throw new Error('В Excel нет листов');
  return parseCastingRows(rows);
}
