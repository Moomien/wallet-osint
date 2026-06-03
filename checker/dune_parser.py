#!/usr/bin/env python3
"""
Скрипт для извлечения Ethereum адресов из файла dune_analytics.txt
"""

import json
import sys
from pathlib import Path


def extract_addresses(input_file: str, output_file: str = 'ethereum_addresses.txt'):
    """
    Извлекает Ethereum адреса из JSON файла Dune Analytics
    
    Args:
        input_file: Путь к входному файлу с JSON данными
        output_file: Путь к выходному файлу для сохранения адресов
    """
    try:
        # Читаем входной файл
        print(f'Читаю файл: {input_file}')
        with open(input_file, 'r', encoding='utf-8') as f:
            content = f.read()
        
        # Парсим JSON
        print('Парсинг JSON...')
        data = json.loads(content)
        
        # Извлекаем адреса из структуры данных
        addresses = []
        
        if 'result' in data and 'rows' in data['result']:
            for row in data['result']['rows']:
                if 'address' in row:
                    address = row['address'] 
                    addresses.append(address)
        
        # Удаляем дубликаты (если есть)
        unique_addresses = list(dict.fromkeys(addresses))
        
        print(f'Найдено адресов: {len(addresses)}')
        print(f'Уникальных адресов: {len(unique_addresses)}')
        
        # Сохраняем адреса в файл
        with open(output_file, 'w', encoding='utf-8') as f:
            for address in unique_addresses:
                f.write(address + '\n')
        
        print(f'Адреса успешно сохранены в файл: {output_file}')
        
        # Выводим статистику
        if 'result' in data and 'rows' in data['result']:
            first_row = data['result']['rows'][0]
            print(f'\nПример данных из первой строки:')
            for key, value in first_row.items():
                print(f'  {key}: {value}')
        
        return unique_addresses
        
    except FileNotFoundError:
        print(f'Ошибка: Файл {input_file} не найден!')
        sys.exit(1)
    except json.JSONDecodeError as e:
        print(f'Ошибка парсинга JSON: {e}')
        sys.exit(1)
    except Exception as e:
        print(f'Неожиданная ошибка: {e}')
        sys.exit(1)


def main():
    """Главная функция"""
    input_file = 'dune_parser.txt'
    output_file = 'addresses.txt'
    
    # Проверяем существование входного файла
    if not Path(input_file).exists():
        print(f'Файл {input_file} не найден в текущей директории!')
        print(f'Текущая директория: {Path.cwd()}')
        sys.exit(1)
    
    # Извлекаем адреса
    addresses = extract_addresses(input_file, output_file)
    
    # Выводим первые 5 адресов для проверки
    if addresses:
        print(f'\nПервые 5 адресов:')
        for i, addr in enumerate(addresses[:5], 1):
            print(f'  {i}. {addr}')


if __name__ == '__main__':
    main()
